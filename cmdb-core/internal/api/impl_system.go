package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// System endpoints
// ---------------------------------------------------------------------------

// GetSystemHealth returns health status of backend dependencies.
// (GET /system/health)
func (s *APIServer) GetSystemHealth(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database
	dbStatus := "ok"
	dbStart := time.Now()
	var one int
	err := s.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
	dbLatency := float32(time.Since(dbStart).Milliseconds())
	if err != nil {
		dbStatus = "error"
	}

	health := SystemHealth{
		Database: &struct {
			LatencyMs *float32 `json:"latency_ms,omitempty"`
			Status    *string  `json:"status,omitempty"`
		}{
			Status:    &dbStatus,
			LatencyMs: &dbLatency,
		},
	}

	response.OK(c, health)
}
