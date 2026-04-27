package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/schedhealth"
	"github.com/gin-gonic/gin"
)

// Wave 9.1 handler test. Three scenarios that matter:
//
//   1. Tracker not wired (typical for unit tests of unrelated handlers)
//      → endpoint returns 200 with all_healthy=true and an empty list
//      so the route shape is stable.
//   2. Schedulers registered but never ticked → status=never_ticked,
//      all_healthy=false (a fresh process should fail readiness for
//      the first interval, matching k8s rolling-deploy expectations).
//   3. Mixed states (one OK, one stale) → status set correctly per
//      scheduler, all_healthy=false.

func TestSchedulerHealth_TrackerNotWired_ReturnsEmptyHealthy(t *testing.T) {
	s := &APIServer{}
	rec := runHandler(t, s.GetSchedulerHealth, http.MethodGet, "/admin/scheduler-health", nil, nil, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Data struct {
			AllHealthy bool              `json:"all_healthy"`
			Schedulers []SchedulerHealth `json:"schedulers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if !body.Data.AllHealthy {
		t.Errorf("nil tracker should report all_healthy=true (route is stable)")
	}
	if len(body.Data.Schedulers) != 0 {
		t.Errorf("nil tracker should return empty list, got %d", len(body.Data.Schedulers))
	}
}

func TestSchedulerHealth_NeverTickedReportsNotHealthy(t *testing.T) {
	tr := schedhealth.New()
	tr.Register("alert_evaluator", time.Minute)
	tr.Register("energy", time.Hour)
	s := &APIServer{schedTracker: tr}

	rec := runHandler(t, s.GetSchedulerHealth, http.MethodGet, "/admin/scheduler-health", nil, nil, nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Data struct {
			AllHealthy bool              `json:"all_healthy"`
			Schedulers []SchedulerHealth `json:"schedulers"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Data.AllHealthy {
		t.Errorf("never-ticked schedulers should not be healthy")
	}
	if len(body.Data.Schedulers) != 2 {
		t.Errorf("schedulers = %d, want 2", len(body.Data.Schedulers))
	}
	for _, item := range body.Data.Schedulers {
		if item.Status != "never_ticked" {
			t.Errorf("scheduler %q status = %q, want never_ticked", item.Name, item.Status)
		}
		if item.LastTickAt != nil {
			t.Errorf("scheduler %q last_tick_at should be nil, got %v", item.Name, item.LastTickAt)
		}
	}
}

func TestSchedulerHealth_MixedStatesReportPerSchedulerVerdict(t *testing.T) {
	clock := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tr := schedhealth.New().WithClock(func() time.Time { return clock })
	tr.Register("recent", time.Minute)
	tr.Register("stale", time.Minute)

	clock = clock.Add(-30 * time.Second) // 30s ago
	tr.Record("recent")
	clock = clock.Add(-150 * time.Second) // pull back to 3min before "now"
	tr.Record("stale")
	clock = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) // restore "now"

	s := &APIServer{schedTracker: tr}
	rec := runHandler(t, s.GetSchedulerHealth, http.MethodGet, "/admin/scheduler-health", nil, nil, nil)

	var body struct {
		Data struct {
			AllHealthy bool              `json:"all_healthy"`
			Schedulers []SchedulerHealth `json:"schedulers"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Data.AllHealthy {
		t.Errorf("mixed states with one stale should not be all_healthy")
	}

	got := map[string]string{}
	for _, item := range body.Data.Schedulers {
		got[item.Name] = string(item.Status)
	}
	if got["recent"] != "ok" {
		t.Errorf("recent: status=%q want ok", got["recent"])
	}
	if got["stale"] != "stale" {
		t.Errorf("stale: status=%q want stale", got["stale"])
	}
}

// Sanity check that the gin handler exposes the recorder helper as
// expected. Not a spec assertion — just guards against the helper file
// disappearing under refactoring.
func TestSchedulerHealth_HandlerSignatureCompiles(t *testing.T) {
	var _ gin.HandlerFunc = (&APIServer{}).GetSchedulerHealth
}
