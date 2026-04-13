// Package eventbus defines the event publishing and subscription interface for
// the CMDB platform and provides a NATS JetStream implementation.
package eventbus

import "context"

// Event represents a single domain event flowing through the bus.
type Event struct {
	Subject  string
	TenantID string
	Payload  []byte
}

// Handler is a callback invoked when an event is received.
type Handler func(ctx context.Context, event Event) error

// Bus is the interface for publishing and subscribing to domain events.
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(subject string, handler Handler) error
	Close() error
}
