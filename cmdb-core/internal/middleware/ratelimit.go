package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig controls the token-bucket parameters for RateLimiter.
type RateLimiterConfig struct {
	// RequestsPerSecond is the sustained request rate allowed per key.
	RequestsPerSecond float64
	// Burst is the maximum number of tokens that may accumulate — i.e. the
	// largest short burst a single key can issue before being throttled.
	Burst int
	// IdleTTL is how long a per-key limiter is kept in memory after its last use.
	// Once exceeded, the janitor evicts it so long-tail traffic can't grow the
	// map unbounded.
	IdleTTL time.Duration
}

// DefaultRateLimiterConfig returns a generous default: 100 rps sustained with a
// 200-token burst per key, and 10-minute idle eviction.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             200,
		IdleTTL:           10 * time.Minute,
	}
}

// RateLimiter is an in-memory per-key token-bucket rate limiter. Keys are
// typically the client IP, or — when an upstream middleware has set user_id
// on the Gin context — the authenticated user ID.
type RateLimiter struct {
	cfg      RateLimiterConfig
	mu       sync.Mutex
	visitors map[string]*rateVisitor
	stop     chan struct{}
}

type rateVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter builds a RateLimiter and starts a background janitor that
// periodically evicts entries idle for longer than cfg.IdleTTL. Call Stop()
// when the server shuts down to release the janitor goroutine.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		cfg:      cfg,
		visitors: make(map[string]*rateVisitor),
		stop:     make(chan struct{}),
	}
	go rl.janitor()
	return rl
}

// Stop halts the background janitor. Existing limiters keep working but idle
// entries will no longer be evicted. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.stop:
		return
	default:
		close(rl.stop)
	}
}

func (rl *RateLimiter) limiterFor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if v, ok := rl.visitors[key]; ok {
		v.lastSeen = time.Now()
		return v.limiter
	}
	lim := rate.NewLimiter(rate.Limit(rl.cfg.RequestsPerSecond), rl.cfg.Burst)
	rl.visitors[key] = &rateVisitor{limiter: lim, lastSeen: time.Now()}
	return lim
}

func (rl *RateLimiter) janitor() {
	interval := rl.cfg.IdleTTL / 2
	if interval <= 0 {
		interval = 30 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-tick.C:
			cutoff := time.Now().Add(-rl.cfg.IdleTTL)
			rl.mu.Lock()
			for k, v := range rl.visitors {
				if v.lastSeen.Before(cutoff) {
					delete(rl.visitors, k)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Middleware returns a Gin handler that checks the per-key token bucket and
// aborts with HTTP 429 when it's empty, setting Retry-After for clients.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rateLimitKey(c)
		if !rl.limiterFor(key).Allow() {
			c.Header("Retry-After", "1")
			response.Err(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}

// rateLimitKey prefers an authenticated user_id (when Auth middleware ran
// first), and falls back to client IP. This means a single misbehaving user
// gets their own bucket even when NAT'd behind a shared IP.
func rateLimitKey(c *gin.Context) string {
	if uid, ok := c.Get("user_id"); ok {
		if s, ok := uid.(string); ok && s != "" {
			return "user:" + s
		}
	}
	return "ip:" + clientIPForRateLimit(c)
}

func clientIPForRateLimit(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}
