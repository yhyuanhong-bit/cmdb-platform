package eventbus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const natsTracerName = "github.com/cmdb-platform/cmdb-core/internal/eventbus"

// NATSBus implements Bus using NATS JetStream.
type NATSBus struct {
	nc   *nats.Conn
	js   jetstream.JetStream
	mu   sync.Mutex
	subs []*nats.Subscription
}

// NewNATSBus connects to NATS with retry logic, creates a JetStream instance,
// and ensures the CMDB stream exists with the required subjects.
func NewNATSBus(url string) (*NATSBus, error) {
	var nc *nats.Conn
	var err error

	for i := 0; i < 5; i++ {
		nc, err = nats.Connect(url,
			nats.RetryOnFailedConnect(true),
			nats.MaxReconnects(10),
			nats.ReconnectWait(2*time.Second),
		)
		if err == nil {
			break
		}
		log.Printf("nats connect attempt %d failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream new: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: "CMDB",
		Subjects: []string{
			"asset.>",
			"alert.>",
			"maintenance.>",
			"import.>",
			"audit.>",
			"prediction.>",
			"config.>",
			"mac_table.>",
		},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create stream: %w", err)
	}

	// Sync stream for edge federation
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: "CMDB_SYNC",
		Subjects: []string{
			"sync.>",
		},
		Retention: jetstream.WorkQueuePolicy,
		MaxAge:    14 * 24 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		log.Printf("WARN: failed to create CMDB_SYNC stream: %v", err)
	}

	return &NATSBus{nc: nc, js: js}, nil
}

// Publish sends an event to the NATS JetStream stream. If the event has a
// non-empty TenantID, it is appended to the subject as a suffix. A producer
// span is opened around the publish, and the current trace context is
// injected into message headers via the configured OTel propagator so the
// subscriber can continue the span as a child.
func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	subject := event.Subject
	if event.TenantID != "" {
		subject = subject + "." + event.TenantID
	}

	tracer := otel.Tracer(natsTracerName)
	ctx, span := tracer.Start(ctx, "nats.publish "+subject,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("nats"),
			semconv.MessagingDestinationName(subject),
			attribute.String("messaging.nats.stream", "CMDB"),
		),
	)
	defer span.End()

	msg := &nats.Msg{Subject: subject, Data: event.Payload, Header: nats.Header{}}
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(msg.Header))

	_, err := b.js.PublishMsg(ctx, msg)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}

// Subscribe registers a handler for the given subject pattern using a plain
// NATS subscription (not a JetStream consumer) for simplicity. Any inbound
// W3C trace context on the message header is extracted and used as the
// parent of the consumer span, so end-to-end traces stitch across the bus.
func (b *NATSBus) Subscribe(subject string, handler Handler) error {
	sub, err := b.nc.Subscribe(subject, func(msg *nats.Msg) {
		parent := otel.GetTextMapPropagator().Extract(context.Background(),
			natsHeaderCarrier(msg.Header))

		tracer := otel.Tracer(natsTracerName)
		ctx, span := tracer.Start(parent, "nats.receive "+msg.Subject,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				semconv.MessagingSystemKey.String("nats"),
				semconv.MessagingDestinationName(msg.Subject),
			),
		)
		defer span.End()

		evt := Event{
			Subject: msg.Subject,
			Payload: msg.Data,
		}
		if err := handler(ctx, evt); err != nil {
			span.RecordError(err)
			log.Printf("event handler error [%s]: %v", msg.Subject, err)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}

	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	return nil
}

// Close unsubscribes all active subscriptions and closes the NATS connection.
func (b *NATSBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subs {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("unsubscribe error: %v", err)
		}
	}
	b.subs = nil
	b.nc.Close()
	return nil
}
