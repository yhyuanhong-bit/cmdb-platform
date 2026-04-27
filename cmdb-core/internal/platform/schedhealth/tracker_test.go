package schedhealth

import (
	"sync"
	"testing"
	"time"
)

// Tests cover the four properties operators rely on:
//   1. Register is idempotent — re-registering preserves last-tick.
//   2. Status computation matches the documented thresholds (1× and 2×).
//   3. Snapshot returns a copy — mutating it doesn't affect the tracker.
//   4. Concurrent Record + Snapshot are race-free under -race.

func TestTracker_RegisterIsIdempotentAndPreservesLastTick(t *testing.T) {
	tr := New()
	tr.Register("alerts", 60*time.Second)
	tr.Record("alerts")
	first := tr.Snapshot()[0].LastTickAt
	if first == nil {
		t.Fatal("expected last_tick_at after Record")
	}
	tr.Register("alerts", 30*time.Second) // re-register with new interval
	snap := tr.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("re-register should not duplicate; got %d entries", len(snap))
	}
	if snap[0].ExpectedInterval != 30*time.Second {
		t.Errorf("interval did not update on re-register: %v", snap[0].ExpectedInterval)
	}
	if snap[0].LastTickAt == nil || !snap[0].LastTickAt.Equal(*first) {
		t.Errorf("last_tick_at clobbered by re-register")
	}
}

func TestTracker_StatusThresholds(t *testing.T) {
	clock := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tr := New().WithClock(func() time.Time { return clock })

	tr.Register("never-ticked", time.Minute)
	tr.Register("recent", time.Minute)
	tr.Register("lagging", time.Minute)
	tr.Register("stale", time.Minute)

	// Plant Record times directly via the clock — bump it forward, then
	// step the clock, etc.
	clock = time.Date(2026, 5, 1, 11, 59, 30, 0, time.UTC) // 30s ago
	tr.Record("recent")
	clock = time.Date(2026, 5, 1, 11, 58, 30, 0, time.UTC) // 90s ago (1.5×)
	tr.Record("lagging")
	clock = time.Date(2026, 5, 1, 11, 57, 0, 0, time.UTC) // 3min ago (3×)
	tr.Record("stale")
	clock = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) // now

	got := map[string]Status{}
	for _, s := range tr.Snapshot() {
		got[s.Name] = s.Status
	}

	if got["never-ticked"] != StatusNeverTicked {
		t.Errorf("never-ticked: got %s", got["never-ticked"])
	}
	if got["recent"] != StatusOK {
		t.Errorf("recent (30s ago, interval 60s): got %s, want ok", got["recent"])
	}
	if got["lagging"] != StatusLagging {
		t.Errorf("lagging (90s ago, interval 60s): got %s, want lagging", got["lagging"])
	}
	if got["stale"] != StatusStale {
		t.Errorf("stale (180s ago, interval 60s): got %s, want stale", got["stale"])
	}
}

func TestTracker_AllHealthyRespectsNeverTickedAndStale(t *testing.T) {
	clock := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tr := New().WithClock(func() time.Time { return clock })

	tr.Register("a", time.Minute)
	tr.Register("b", time.Minute)

	// Both never-ticked → not healthy.
	if tr.AllHealthy() {
		t.Errorf("never-ticked schedulers should not be healthy")
	}

	// Record both at "now" → healthy.
	tr.Record("a")
	tr.Record("b")
	if !tr.AllHealthy() {
		t.Errorf("just-ticked schedulers should be healthy")
	}

	// Push clock forward past 2× interval → stale → not healthy.
	clock = clock.Add(150 * time.Second)
	if tr.AllHealthy() {
		t.Errorf("stale scheduler should not be healthy")
	}

	// Record one of them; the other is still stale → not healthy.
	tr.Record("a")
	if tr.AllHealthy() {
		t.Errorf("one stale scheduler should fail AllHealthy")
	}

	// Record the other → healthy again.
	tr.Record("b")
	if !tr.AllHealthy() {
		t.Errorf("all just-ticked → should be healthy")
	}
}

func TestTracker_SnapshotIsCopy(t *testing.T) {
	tr := New()
	tr.Register("a", time.Minute)
	tr.Record("a")
	snap1 := tr.Snapshot()
	if len(snap1) != 1 {
		t.Fatalf("got %d entries", len(snap1))
	}
	// Mutate the returned snapshot — must NOT affect a subsequent
	// Snapshot call.
	snap1[0].Name = "mutated"
	snap2 := tr.Snapshot()
	if snap2[0].Name == "mutated" {
		t.Errorf("Snapshot returned a live reference; mutation leaked into the tracker")
	}
}

func TestTracker_ConcurrentRecordAndSnapshotIsRaceFree(t *testing.T) {
	// Run with `go test -race`; this test exercises both write and read
	// paths simultaneously. Without the mutex this would trip the race
	// detector.
	tr := New()
	for _, name := range []string{"a", "b", "c", "d"} {
		tr.Register(name, time.Second)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	for _, name := range []string{"a", "b", "c", "d"} {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					tr.Record(n)
				}
			}
		}(name)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = tr.Snapshot()
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestTracker_RegisterRejectsInvalidInputs(t *testing.T) {
	tr := New()
	tr.Register("zero", 0)
	tr.Register("negative", -1*time.Second)
	tr.Register("", time.Minute) // empty name
	if len(tr.Snapshot()) != 0 {
		t.Errorf("invalid registers should be silently ignored, got %d entries", len(tr.Snapshot()))
	}
}
