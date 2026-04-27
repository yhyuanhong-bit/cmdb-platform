package api

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/cmdb-platform/cmdb-core/internal/platform/schedhealth"
	"github.com/gin-gonic/gin"
)

// GetSchedulerHealth — GET /admin/scheduler-health
//
// Reports last-tick timestamps for every background scheduler the
// platform runs. External readiness monitors use this to detect a
// stuck loop that wouldn't show up in /readyz (which only checks DB,
// Redis, NATS connectivity, not whether scheduled jobs are still
// firing).
func (s *APIServer) GetSchedulerHealth(c *gin.Context) {
	if s.schedTracker == nil {
		// Tracker not wired — typical for unit tests of the handler
		// in isolation. Return an empty report rather than 500 so
		// the endpoint shape is stable.
		response.OK(c, gin.H{"all_healthy": true, "schedulers": []SchedulerHealth{}})
		return
	}
	snap := s.schedTracker.Snapshot()
	out := make([]SchedulerHealth, 0, len(snap))
	for _, item := range snap {
		out = append(out, toAPISchedulerHealth(item))
	}
	response.OK(c, gin.H{
		"all_healthy": s.schedTracker.AllHealthy(),
		"schedulers":  out,
	})
}

func toAPISchedulerHealth(s schedhealth.Snapshot) SchedulerHealth {
	out := SchedulerHealth{
		Name:   s.Name,
		Status: SchedulerHealthStatus(s.Status),
	}
	intervalSecs := int64(s.ExpectedInterval.Seconds())
	out.ExpectedIntervalSeconds = &intervalSecs
	if s.LastTickAt != nil {
		t := *s.LastTickAt
		out.LastTickAt = &t
	}
	if s.SecondsSinceTick != nil {
		v := *s.SecondsSinceTick
		out.SecondsSinceTick = &v
	}
	return out
}
