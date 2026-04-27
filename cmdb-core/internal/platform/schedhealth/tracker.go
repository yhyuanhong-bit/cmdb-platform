// Package schedhealth tracks last-tick timestamps for the platform's
// background schedulers (alert evaluator, energy aggregator, predictive
// scanner, workflow tickers) so the operator's readiness probe can tell
// "the API is up but the energy aggregator hasn't ticked in 6 hours" —
// a class of failure that's invisible from outside the process.
//
// The tracker is deliberately simple: an in-memory map from a scheduler
// name to its last-tick wall-clock time, plus an "expected interval" so
// the read path can compute a staleness verdict without each caller
// reinventing the threshold. There's no persistence — a process
// restart legitimately resets the timestamps because no tick has run
// in the new process yet.
//
// Design notes:
//   - All operations are safe under concurrent access. Multiple
//     schedulers ticking at once + multiple HTTP handlers reading must
//     not race; the mutex is fine because reads are infrequent and
//     writes are once-per-tick.
//   - Snapshot returns a copy so callers can iterate without holding
//     the lock and without seeing partial writes.
//   - Register is idempotent: the same scheduler can call it on every
//     start without producing duplicates. The expected_interval from
//     the most recent Register wins, which is the behaviour you want
//     when a config reload changes the cadence.
package schedhealth

import (
	"sync"
	"time"
)

// Status is the staleness verdict for a scheduler.
type Status string

const (
	// StatusOK — the last tick was within ExpectedInterval.
	StatusOK Status = "ok"
	// StatusLagging — last tick is between 1× and 2× ExpectedInterval.
	// Could be jitter, could be the start of a problem.
	StatusLagging Status = "lagging"
	// StatusStale — last tick is older than 2× ExpectedInterval.
	// Tighter alarm than Lagging; readiness probes should fail this.
	StatusStale Status = "stale"
	// StatusNeverTicked — the scheduler has registered itself but has
	// never recorded a tick. Common right after a restart and ok for
	// the first ExpectedInterval; after that it's effectively stale.
	StatusNeverTicked Status = "never_ticked"
)

// Snapshot is the read-side type returned by Tracker.Snapshot. Each
// entry includes the computed Status so the caller doesn't need to
// know about ExpectedInterval — kept separate from the live map so
// HTTP handlers don't have to take the tracker's lock.
type Snapshot struct {
	Name             string        `json:"name"`
	ExpectedInterval time.Duration `json:"expected_interval_seconds"`
	LastTickAt       *time.Time    `json:"last_tick_at,omitempty"`
	SecondsSinceTick *int64        `json:"seconds_since_tick,omitempty"`
	Status           Status        `json:"status"`
}

// Tracker is the singleton an operator wires once at startup and hands
// to every scheduler. Schedulers call Register once at start and Record
// at the top of each tick.
type Tracker struct {
	mu    sync.RWMutex
	now   func() time.Time // injectable for tests
	state map[string]*entry
}

type entry struct {
	expectedInterval time.Duration
	lastTickAt       time.Time
}

// New returns a Tracker using time.Now as the clock.
func New() *Tracker {
	return &Tracker{now: time.Now, state: map[string]*entry{}}
}

// WithClock replaces the clock with the supplied function. Used by
// tests to drive the tracker forward deterministically without
// time.Sleep.
func (t *Tracker) WithClock(now func() time.Time) *Tracker {
	t.mu.Lock()
	t.now = now
	t.mu.Unlock()
	return t
}

// Register declares a scheduler to the tracker with its expected tick
// cadence. Idempotent — calling it twice with the same name updates
// the interval and preserves any existing lastTickAt. Called once
// from each scheduler at startup before its loop begins.
func (t *Tracker) Register(name string, expectedInterval time.Duration) {
	if name == "" || expectedInterval <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.state[name]; ok {
		e.expectedInterval = expectedInterval
		return
	}
	t.state[name] = &entry{expectedInterval: expectedInterval}
}

// Record stamps the scheduler's last-tick time as "now". Safe to call
// from any goroutine; missing Register call is a no-op (we'd rather
// silently ignore than panic the scheduler on startup ordering bugs).
func (t *Tracker) Record(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.state[name]; ok {
		e.lastTickAt = t.now()
	}
}

// Snapshot returns the current state for every registered scheduler,
// with Status computed against the tracker's clock. The returned slice
// is a copy — callers can iterate without holding the lock.
func (t *Tracker) Snapshot() []Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := t.now()
	out := make([]Snapshot, 0, len(t.state))
	for name, e := range t.state {
		s := Snapshot{
			Name:             name,
			ExpectedInterval: e.expectedInterval,
		}
		if e.lastTickAt.IsZero() {
			s.Status = StatusNeverTicked
		} else {
			t := e.lastTickAt
			s.LastTickAt = &t
			elapsed := now.Sub(e.lastTickAt)
			secs := int64(elapsed.Seconds())
			s.SecondsSinceTick = &secs
			switch {
			case elapsed >= 2*e.expectedInterval:
				s.Status = StatusStale
			case elapsed >= e.expectedInterval:
				s.Status = StatusLagging
			default:
				s.Status = StatusOK
			}
		}
		out = append(out, s)
	}
	return out
}

// AllHealthy returns true when every registered scheduler is OK or
// LaggingButRecent. Used by readiness probes that want a single
// boolean. NeverTicked counts as not-healthy after the registration
// has been around long enough — but the tracker can't know how long
// "long enough" is without external state, so we treat
// NeverTicked + Stale as not-healthy unconditionally. A fresh process
// will fail readiness for one tick interval, which matches what a
// k8s rolling deploy expects (gradual cutover).
func (t *Tracker) AllHealthy() bool {
	for _, s := range t.Snapshot() {
		if s.Status == StatusStale || s.Status == StatusNeverTicked {
			return false
		}
	}
	return true
}
