package api

//tenantlint:allow-direct-pool — health-check SELECT 1 is deliberately cross-tenant

import (
	"context"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// System endpoints
// ---------------------------------------------------------------------------

// GetSystemHealth returns health status of backend dependencies. Mirrors
// the /readyz component checks (database + redis + nats) so the operator
// SystemHealth page can surface real infra state instead of a hardcoded
// "OPERATIONAL" badge — audit finding H9, 2026-04-28.
//
// (GET /system/health)
func (s *APIServer) GetSystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	health := SystemHealth{}

	// Database
	if s.pool != nil {
		dbStart := time.Now()
		var one int
		err := s.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
		dbLatency := float32(time.Since(dbStart).Milliseconds())
		dbStatus := "ok"
		if err != nil {
			dbStatus = "error"
		}
		health.Database = &struct {
			LatencyMs *float32 `json:"latency_ms,omitempty"`
			Status    *string  `json:"status,omitempty"`
		}{
			Status:    &dbStatus,
			LatencyMs: &dbLatency,
		}
	}

	// Redis
	if s.redis != nil {
		redisStart := time.Now()
		err := s.redis.Ping(ctx).Err()
		redisLatency := float32(time.Since(redisStart).Milliseconds())
		redisStatus := "ok"
		if err != nil {
			redisStatus = "error"
		}
		health.Redis = &struct {
			LatencyMs *float32 `json:"latency_ms,omitempty"`
			Status    *string  `json:"status,omitempty"`
		}{
			Status:    &redisStatus,
			LatencyMs: &redisLatency,
		}
	}

	// NATS — connection-state only, no latency probe (the JetStream client
	// caches connection state and a synthetic ping is overkill here).
	if s.nats != nil {
		connected := s.nats.IsConnected()
		natsStatus := "ok"
		if !connected {
			natsStatus = "error"
		}
		health.Nats = &struct {
			Connected *bool   `json:"connected,omitempty"`
			Status    *string `json:"status,omitempty"`
		}{
			Status:    &natsStatus,
			Connected: &connected,
		}
	}

	response.OK(c, health)
}
