package eventbus

import (
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/propagation"
)

// natsHeaderCarrier adapts nats.Header to propagation.TextMapCarrier
// so trace context can ride alongside every JetStream publish and be
// reconstructed on the subscriber side.
type natsHeaderCarrier nats.Header

func (c natsHeaderCarrier) Get(key string) string {
	if len(c) == 0 {
		return ""
	}
	return nats.Header(c).Get(key)
}

func (c natsHeaderCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = natsHeaderCarrier(nil)
