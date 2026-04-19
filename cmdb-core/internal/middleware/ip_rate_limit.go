package middleware

import (
	"net/http"
	"sync"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter is a per-IP token-bucket limiter intended for unauthenticated
// endpoints (login, refresh) where user_id is not yet available. The bucket
// permits up to `perMinute` requests in a burst, refilling at the same
// effective rate. Separate IPs get independent buckets.
//
// This is intentionally a narrow, dependency-free implementation distinct
// from the generic RateLimiter in ratelimit.go: login/refresh need to key
// strictly on IP (no user_id exists before auth completes) and need a
// per-minute rather than per-second budget so we can express "5/min" cleanly.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// NewIPRateLimiter returns a limiter allowing `perMinute` requests per IP
// with a matching burst, so a client can spend their full minute's budget
// immediately after a fresh start.
func NewIPRateLimiter(perMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(float64(perMinute) / 60.0),
		burst:    perMinute,
	}
}

// get returns the limiter for ip, creating one on first use.
func (i *IPRateLimiter) get(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	if lim, ok := i.limiters[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(i.rps, i.burst)
	i.limiters[ip] = lim
	return lim
}

// Middleware returns a Gin handler that rejects with 429 when the IP's
// token bucket is empty and otherwise forwards to the next handler.
func (i *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !i.get(c.ClientIP()).Allow() {
			c.Header("Retry-After", "60")
			response.Err(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}
