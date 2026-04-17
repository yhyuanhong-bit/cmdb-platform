package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestRouter(t *testing.T, cfg RateLimiterConfig, preMiddleware ...gin.HandlerFunc) (*gin.Engine, *RateLimiter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(cfg)
	t.Cleanup(rl.Stop)
	r := gin.New()
	for _, m := range preMiddleware {
		r.Use(m)
	}
	r.Use(rl.Middleware())
	r.GET("/t", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r, rl
}

func sendFrom(router *gin.Engine, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	router.ServeHTTP(w, req)
	return w
}

func TestRateLimiter_AllowsBurstThenRejects(t *testing.T) {
	router, _ := newTestRouter(t, RateLimiterConfig{
		RequestsPerSecond: 0.5, // refill too slow to matter in test
		Burst:             3,
		IdleTTL:           time.Minute,
	})

	tests := []struct {
		name       string
		wantStatus int
	}{
		{"burst-1", http.StatusOK},
		{"burst-2", http.StatusOK},
		{"burst-3", http.StatusOK},
		{"exceeds", http.StatusTooManyRequests},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := sendFrom(router, "10.0.0.1:1000", nil)
			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d (body=%s)", tc.wantStatus, w.Code, w.Body.String())
			}
			if tc.wantStatus == http.StatusTooManyRequests && w.Header().Get("Retry-After") == "" {
				t.Error("expected Retry-After header on 429")
			}
		})
	}
}

func TestRateLimiter_IsolatesKeysByIP(t *testing.T) {
	router, _ := newTestRouter(t, RateLimiterConfig{
		RequestsPerSecond: 0.001,
		Burst:             1,
		IdleTTL:           time.Minute,
	})

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"ip-A first", "10.0.0.2:1000", http.StatusOK},
		{"ip-A second blocked", "10.0.0.2:1000", http.StatusTooManyRequests},
		{"ip-B independent bucket", "10.0.0.3:1000", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := sendFrom(router, tc.ip, nil)
			if w.Code != tc.wantStatus {
				t.Errorf("IP %s: expected %d, got %d", tc.ip, tc.wantStatus, w.Code)
			}
		})
	}
}

func TestRateLimiter_PrefersUserIDOverIP(t *testing.T) {
	// Stub auth that lifts X-User-ID into the Gin context. This mimics the
	// real Auth middleware running before rate limit in the chain.
	stubAuth := func(c *gin.Context) {
		if uid := c.GetHeader("X-User-ID"); uid != "" {
			c.Set("user_id", uid)
		}
		c.Next()
	}
	router, _ := newTestRouter(t, RateLimiterConfig{
		RequestsPerSecond: 0.001,
		Burst:             1,
		IdleTTL:           time.Minute,
	}, stubAuth)

	tests := []struct {
		name       string
		ip         string
		userID     string
		wantStatus int
	}{
		{"user-a first from shared IP", "10.0.0.4:1000", "user-a", http.StatusOK},
		{"user-b first from same IP", "10.0.0.4:1000", "user-b", http.StatusOK},
		{"user-a repeat blocked", "10.0.0.4:1000", "user-a", http.StatusTooManyRequests},
		{"user-b repeat blocked", "10.0.0.4:1000", "user-b", http.StatusTooManyRequests},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := sendFrom(router, tc.ip, map[string]string{"X-User-ID": tc.userID})
			if w.Code != tc.wantStatus {
				t.Errorf("user %s: expected %d, got %d", tc.userID, tc.wantStatus, w.Code)
			}
		})
	}
}

func TestRateLimiter_JanitorEvictsIdle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		IdleTTL:           100 * time.Millisecond,
	})
	t.Cleanup(rl.Stop)

	_ = rl.limiterFor("key-1")
	rl.mu.Lock()
	size := len(rl.visitors)
	rl.mu.Unlock()
	if size != 1 {
		t.Fatalf("expected 1 visitor after touch, got %d", size)
	}

	// Janitor ticks every IdleTTL/2 = 50ms; entries older than IdleTTL (100ms)
	// are dropped. 250ms covers several ticks and ensures a full eviction pass.
	time.Sleep(250 * time.Millisecond)

	rl.mu.Lock()
	size = len(rl.visitors)
	rl.mu.Unlock()
	if size != 0 {
		t.Errorf("expected janitor to evict idle visitor, got %d remaining", size)
	}
}

func TestRateLimiter_StopIsIdempotent(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())
	rl.Stop()
	rl.Stop() // must not panic from closing a closed channel
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	router, _ := newTestRouter(t, RateLimiterConfig{
		RequestsPerSecond: 1000,
		Burst:             1000,
		IdleTTL:           time.Minute,
	})

	var wg sync.WaitGroup
	const workers = 20
	const perWorker = 50
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "10.0.1." + string(rune('0'+id%10)) + ":1000"
			for j := 0; j < perWorker; j++ {
				_ = sendFrom(router, ip, nil)
			}
		}(i)
	}
	wg.Wait()
	// Test primarily exists to catch data races under -race; no status
	// assertion because exact 429 count depends on scheduling.
}
