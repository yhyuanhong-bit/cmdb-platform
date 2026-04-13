package middleware

import (
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// SyncGateMiddleware blocks API requests while Edge is performing initial sync.
func SyncGateMiddleware(initialSyncDone *atomic.Bool, deployMode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deployMode != "edge" {
			c.Next()
			return
		}
		if initialSyncDone.Load() {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if path == "/readyz" || path == "/healthz" || path == "/metrics" || strings.HasPrefix(path, "/api/v1/sync/") {
			c.Next()
			return
		}
		c.Header("Retry-After", "30")
		c.JSON(503, gin.H{
			"error": gin.H{
				"code":    "SYNC_IN_PROGRESS",
				"message": "Edge node is performing initial sync. Please wait.",
			},
		})
		c.Abort()
	}
}
