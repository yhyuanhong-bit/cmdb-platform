package eventbus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

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
// non-empty TenantID, it is appended to the subject as a suffix.
func (b *NATSBus) Publish(ctx context.Context, event Event) error {
	subject := event.Subject
	if event.TenantID != "" {
		subject = subject + "." + event.TenantID
	}
	_, err := b.js.Publish(ctx, subject, event.Payload)
	if err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}

// Subscribe registers a handler for the given subject pattern using a plain
// NATS subscription (not a JetStream consumer) for simplicity.
func (b *NATSBus) Subscribe(subject string, handler Handler) error {
	sub, err := b.nc.Subscribe(subject, func(msg *nats.Msg) {
		evt := Event{
			Subject: msg.Subject,
			Payload: msg.Data,
		}
		if err := handler(context.Background(), evt); err != nil {
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
