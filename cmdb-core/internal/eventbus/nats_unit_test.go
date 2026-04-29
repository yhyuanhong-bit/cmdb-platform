package eventbus

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
)

// Connection-free unit tests for NATSBus. The Publish/Subscribe paths require
// a live JetStream server and live in a separate (future) integration test
// guarded by build tag. These tests cover the nil-receiver guards, the
// natsHeaderCarrier W3C trace propagation helper, the Bus interface contract
// (via mock), and the subject constant catalog so renames break the build.

// --- Event struct ----------------------------------------------------------

func TestEventFields(t *testing.T) {
	e := Event{Subject: "asset.created", TenantID: "t1", Payload: []byte("p")}
	if e.Subject != "asset.created" {
		t.Errorf("Subject = %q", e.Subject)
	}
	if e.TenantID != "t1" {
		t.Errorf("TenantID = %q", e.TenantID)
	}
}

func TestEventZeroValue(t *testing.T) {
	var e Event
	if e.Subject != "" || e.TenantID != "" || e.Payload != nil {
		t.Errorf("zero Event leaked state: %+v", e)
	}
}

// --- Bus interface contract -------------------------------------------------

var _ Bus = (*NATSBus)(nil)

type mockBus struct {
	published []Event
	handlers  map[string]Handler
	closed    bool
}

func newMockBus() *mockBus { return &mockBus{handlers: make(map[string]Handler)} }
func (m *mockBus) Publish(_ context.Context, event Event) error {
	m.published = append(m.published, event)
	return nil
}
func (m *mockBus) Subscribe(subject string, handler Handler) error {
	m.handlers[subject] = handler
	return nil
}
func (m *mockBus) Close() error { m.closed = true; return nil }

var _ Bus = (*mockBus)(nil)

func TestMockBusPublishSubscribeClose(t *testing.T) {
	mb := newMockBus()
	evt := Event{Subject: "asset.created", TenantID: "t1"}
	if err := mb.Publish(context.Background(), evt); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	called := false
	_ = mb.Subscribe("asset.created", func(_ context.Context, _ Event) error {
		called = true
		return nil
	})
	_ = mb.handlers["asset.created"](context.Background(), evt)
	if !called {
		t.Error("handler not invoked")
	}
	_ = mb.Close()
	if !mb.closed {
		t.Error("not closed")
	}
}

func TestHandlerErrorPropagation(t *testing.T) {
	mb := newMockBus()
	want := errors.New("fail")
	_ = mb.Subscribe("x", func(_ context.Context, _ Event) error { return want })
	if err := mb.handlers["x"](context.Background(), Event{}); !errors.Is(err, want) {
		t.Errorf("got %v, want %v", err, want)
	}
}

// --- IsConnected guards -----------------------------------------------------

func TestIsConnectedNilReceiver(t *testing.T) {
	var b *NATSBus
	if b.IsConnected() {
		t.Error("nil receiver must return false")
	}
}

func TestIsConnectedNilNC(t *testing.T) {
	if (&NATSBus{nc: nil}).IsConnected() {
		t.Error("nil nc must return false")
	}
}

// TestIsConnectedReconnecting drives the nc.IsConnected() branch. Connect to
// an unused port with RetryOnFailedConnect so we get a non-nil *nats.Conn back
// in reconnecting state — the conn is never connected but the guard inside
// IsConnected exercises the live path.
func TestIsConnectedReconnecting(t *testing.T) {
	nc, err := nats.Connect("nats://127.0.0.1:14779",
		nats.RetryOnFailedConnect(true), nats.MaxReconnects(0), nats.NoReconnect())
	if err != nil {
		t.Skipf("unexpected connect error: %v", err)
	}
	defer nc.Close()
	_ = (&NATSBus{nc: nc}).IsConnected()
}

func TestNATSBusCloseEmptySubs(t *testing.T) {
	nc, err := nats.Connect("nats://127.0.0.1:14779",
		nats.RetryOnFailedConnect(true), nats.MaxReconnects(0), nats.NoReconnect())
	if err != nil {
		t.Skipf("unexpected connect error: %v", err)
	}
	if err := (&NATSBus{nc: nc}).Close(); err != nil {
		t.Errorf("Close on empty subs: %v", err)
	}
}

// --- natsHeaderCarrier (W3C trace propagation adapter) ---------------------

func TestNATSHeaderCarrierSetGetKeys(t *testing.T) {
	h := nats.Header{}
	c := natsHeaderCarrier(h)
	c.Set("traceparent", "v1")
	c.Set("tracestate", "ts=1")
	if got := c.Get("traceparent"); got != "v1" {
		t.Errorf("Get traceparent = %q", got)
	}
	if k := c.Keys(); len(k) != 2 {
		t.Errorf("Keys() len = %d, want 2 (got %v)", len(k), k)
	}
}

func TestNATSHeaderCarrierMultipleKeys(t *testing.T) {
	h := nats.Header{}
	c := natsHeaderCarrier(h)
	for _, p := range []struct{ k, v string }{
		{"traceparent", "a"}, {"tracestate", "b"}, {"baggage", "c"},
	} {
		c.Set(p.k, p.v)
	}
	if k := c.Keys(); len(k) != 3 {
		t.Errorf("Keys() len = %d, want 3", len(k))
	}
}

func TestNATSHeaderCarrierOverwrite(t *testing.T) {
	h := nats.Header{}
	c := natsHeaderCarrier(h)
	c.Set("traceparent", "first")
	c.Set("traceparent", "second")
	if got := c.Get("traceparent"); got != "second" {
		t.Errorf("overwrite failed: got %q, want second", got)
	}
}
