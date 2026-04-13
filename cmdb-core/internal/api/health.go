package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler provides health and readiness endpoints.
type HealthHandler struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	return &HealthHandler{pool: pool, redis: redis}
}

// Liveness returns 200 if the process is alive. Used for K8s liveness probe.
// Does NOT check dependencies — a restart won't fix a DB outage.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// Readiness checks all critical dependencies. Used for K8s readiness probe.
// Returns 503 if any dependency is down — traffic should not be routed here.
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	healthy := true

	// Check PostgreSQL
	dbStart := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		checks["database"] = gin.H{"status": "down", "error": err.Error()}
		healthy = false
	} else {
		checks["database"] = gin.H{"status": "up", "latency_ms": time.Since(dbStart).Milliseconds()}
	}

	// Check Redis
	if h.redis != nil {
		redisStart := time.Now()
		if err := h.redis.Ping(ctx).Err(); err != nil {
			checks["redis"] = gin.H{"status": "down", "error": err.Error()}
			healthy = false
		} else {
			checks["redis"] = gin.H{"status": "up", "latency_ms": time.Since(redisStart).Milliseconds()}
		}
	} else {
		checks["redis"] = gin.H{"status": "not_configured"}
	}

	status := 200
	statusText := "ready"
	if !healthy {
		status = 503
		statusText = "not_ready"
	}

	c.JSON(status, gin.H{
		"status": statusText,
		"checks": checks,
	})
}
