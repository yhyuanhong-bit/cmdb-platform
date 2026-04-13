package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSyncGateMiddleware_CentralMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var done atomic.Bool
	done.Store(false)
	r := gin.New()
	r.Use(SyncGateMiddleware(&done, "cloud"))
	r.GET("/api/v1/assets", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assets", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Central mode should pass through, got %d", w.Code)
	}
}

func TestSyncGateMiddleware_EdgeSyncing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var done atomic.Bool
	done.Store(false)
	r := gin.New()
	r.Use(SyncGateMiddleware(&done, "edge"))
	r.GET("/api/v1/assets", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assets", nil)
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Errorf("Edge syncing should return 503, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "30" {
		t.Errorf("expected Retry-After: 30, got %q", w.Header().Get("Retry-After"))
	}
}

func TestSyncGateMiddleware_EdgeSyncDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var done atomic.Bool
	done.Store(true)
	r := gin.New()
	r.Use(SyncGateMiddleware(&done, "edge"))
	r.GET("/api/v1/assets", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/assets", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("Edge sync done should pass through, got %d", w.Code)
	}
}

func TestSyncGateMiddleware_AllowReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var done atomic.Bool
	done.Store(false)
	r := gin.New()
	r.Use(SyncGateMiddleware(&done, "edge"))
	r.GET("/readyz", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/readyz", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("/readyz should always pass through, got %d", w.Code)
	}
}

func TestSyncGateMiddleware_AllowSyncEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var done atomic.Bool
	done.Store(false)
	r := gin.New()
	r.Use(SyncGateMiddleware(&done, "edge"))
	r.GET("/api/v1/sync/state", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/sync/state", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("/api/v1/sync/* should always pass through, got %d", w.Code)
	}
}
