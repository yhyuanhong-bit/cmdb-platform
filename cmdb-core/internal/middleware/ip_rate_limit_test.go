package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestIPRateLimiter_Blocks6thRequest verifies that once the per-IP budget is
// exhausted the limiter returns 429, and that distinct IPs get independent
// buckets (so one noisy client does not starve another).
func TestIPRateLimiter_Blocks6thRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewIPRateLimiter(5)

	// Fire 5 requests from the same IP — all should pass.
	for i := 1; i <= 5; i++ {
		rec := dispatch(t, limiter, "1.2.3.4")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d from 1.2.3.4: status = %d, want 200", i, rec.Code)
		}
	}

	// 6th request should be rate-limited.
	rec := dispatch(t, limiter, "1.2.3.4")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th request: status = %d, want 429", rec.Code)
	}

	// A different IP is tracked independently.
	rec = dispatch(t, limiter, "5.6.7.8")
	if rec.Code != http.StatusOK {
		t.Fatalf("request from 5.6.7.8: status = %d, want 200 (independent bucket)", rec.Code)
	}
}

// TestIPRateLimiter_SeparateBuckets ensures the limiter map reuses the same
// bucket for repeated requests from the same IP instead of creating fresh
// buckets on every hit.
func TestIPRateLimiter_SeparateBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewIPRateLimiter(2)

	// Two requests from IP A.
	for i := 0; i < 2; i++ {
		if rec := dispatch(t, limiter, "10.0.0.1"); rec.Code != http.StatusOK {
			t.Fatalf("A request %d: status = %d, want 200", i, rec.Code)
		}
	}
	// 3rd from A is blocked.
	if rec := dispatch(t, limiter, "10.0.0.1"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("A request 3: status = %d, want 429", rec.Code)
	}
	// IP B still has a full bucket.
	if rec := dispatch(t, limiter, "10.0.0.2"); rec.Code != http.StatusOK {
		t.Fatalf("B request 1: status = %d, want 200", rec.Code)
	}
}

// dispatch runs the limiter middleware once for the given client IP and
// returns the recorded response. A trivial OK handler runs after the
// middleware when the request is not aborted.
func dispatch(t *testing.T, limiter *IPRateLimiter, clientIP string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = clientIP + ":12345"
	c.Request = req

	mw := limiter.Middleware()
	mw(c)
	if !c.IsAborted() {
		c.Status(http.StatusOK)
	}
	return rec
}
