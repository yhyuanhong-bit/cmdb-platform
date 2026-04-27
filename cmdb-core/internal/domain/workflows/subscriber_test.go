package workflows

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
)

// stubBus records every Subscribe call so the subject set can be
// asserted. Publish is a no-op (workflows.Register only subscribes).
// Close returns nil.
type stubBus struct {
	mu        sync.Mutex
	subjects  []string
	subscribe func(subject string, h eventbus.Handler) error
}

func (b *stubBus) Publish(_ context.Context, _ eventbus.Event) error { return nil }
func (b *stubBus) Subscribe(subject string, h eventbus.Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subjects = append(b.subjects, subject)
	if b.subscribe != nil {
		return b.subscribe(subject, h)
	}
	return nil
}
func (b *stubBus) Close() error { return nil }

// TestNew_ConstructsSubscriberWithSystemUsers proves the constructor
// wires up the SystemUserResolver alongside the other dependencies. We
// can't dial a real DB here, but Resolver construction is pure (just
// stores its arguments + a sentinel TTL), so a nil pool/queries is
// sufficient to exercise the field assignment paths.
func TestNew_ConstructsSubscriberWithSystemUsers(t *testing.T) {
	t.Parallel()
	bus := &stubBus{}
	w := New(nil, nil, bus, nil, nil)
	if w == nil {
		t.Fatal("New returned nil subscriber")
	}
	if w.bus != bus {
		t.Errorf("bus not wired: got %v, want %v", w.bus, bus)
	}
	if w.systemUsers == nil {
		t.Error("systemUsers resolver not wired by New")
	}
	if w.qualitySvc != nil {
		t.Error("qualitySvc must be nil until WithQualityScanner is called")
	}
}

// TestWithQualityScanner_InjectsAndChains proves the option chains and
// flips qualitySvc to the provided dependency. Returning the receiver
// is part of the public contract — `New(...).WithQualityScanner(...)`
// is the documented construction idiom.
func TestWithQualityScanner_InjectsAndChains(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{}
	scanner := &fakeQualityScanner{}

	got := w.WithQualityScanner(scanner)
	if got != w {
		t.Errorf("WithQualityScanner must return receiver for chaining; got %v want %v", got, w)
	}
	if w.qualitySvc != scanner {
		t.Errorf("qualitySvc not set; got %v want %v", w.qualitySvc, scanner)
	}
}

// TestRegister_SubscribesAllProductionSubjects locks down the exact
// set of event subjects the workflow subscriber listens to. Adding a
// new Subscribe call without updating this test is the intended
// failure mode — operators should review event-coverage changes the
// same way they review API surface changes.
func TestRegister_SubscribesAllProductionSubjects(t *testing.T) {
	t.Parallel()
	bus := &stubBus{}
	w := &WorkflowSubscriber{bus: bus}
	w.Register()

	got := append([]string(nil), bus.subjects...)
	sort.Strings(got)

	want := []string{
		"alert.fired",
		eventbus.SubjectAssetCreated,
		eventbus.SubjectBMCDefaultPassword,
		eventbus.SubjectImportCompleted,
		eventbus.SubjectInventoryTaskCompleted,
		eventbus.SubjectOrderTransitioned,
		eventbus.SubjectScanDifferencesDetected,
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("subscribed subject count mismatch: got %d %v, want %d %v",
			len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("subject[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}
