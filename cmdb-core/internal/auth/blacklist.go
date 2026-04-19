// Package auth provides token revocation primitives that layer on top of the
// JWT middleware.
//
// The Blacklist type stores revoked jti claims in Redis with a TTL equal to
// the token's remaining lifetime so entries self-expire — no reaper job
// required. The middleware consults the blacklist on every authenticated
// request and rejects matching tokens with TOKEN_REVOKED.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultPrefix keeps blacklist keys in their own Redis namespace so they do
// not clash with refresh tokens, permission caches, or other app keys.
const defaultPrefix = "jti_blacklist:"

// redisDoer is the narrow subset of *redis.Client operations the Blacklist
// actually uses. Exposed so tests can substitute an in-memory fake without
// pulling in a real Redis process.
type redisDoer interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
}

// Blacklist revokes access tokens by jti. Stored in Redis with TTL =
// remaining lifetime of the token, so entries self-expire.
type Blacklist struct {
	rdb    redisDoer
	prefix string
}

// NewBlacklist returns a Blacklist that writes under the default key prefix.
// Passing a nil *redis.Client yields a Blacklist whose IsRevoked is a no-op
// and whose Revoke returns an error — callers can still install it at startup
// when Redis is not yet available and the middleware will fail-open with a
// warning rather than crash.
func NewBlacklist(rdb *redis.Client) *Blacklist {
	if rdb == nil {
		// Avoid the typed-nil-interface trap: store the concrete nil as an
		// untyped nil so (b.rdb == nil) in Revoke/IsRevoked actually holds.
		return &Blacklist{rdb: nil, prefix: defaultPrefix}
	}
	return &Blacklist{rdb: rdb, prefix: defaultPrefix}
}

// newBlacklistWithDoer constructs a Blacklist with a custom redisDoer. Used
// by tests to inject a fake client. Unexported so external packages stay on
// the stable NewBlacklist signature.
func newBlacklistWithDoer(rdb redisDoer, prefix string) *Blacklist {
	if prefix == "" {
		prefix = defaultPrefix
	}
	return &Blacklist{rdb: rdb, prefix: prefix}
}

// Revoke marks a jti as blacklisted until the token's expiry. Revoking a
// jti whose expiry is already in the past is a no-op so we don't store
// permanently-stale entries.
func (b *Blacklist) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if b == nil || b.rdb == nil {
		return errors.New("auth.Blacklist: not configured")
	}
	if jti == "" {
		return errors.New("auth.Blacklist: empty jti")
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	return b.rdb.Set(ctx, b.prefix+jti, "1", ttl).Err()
}

// IsRevoked returns true if the jti has been revoked.
func (b *Blacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if b == nil || b.rdb == nil {
		return false, nil
	}
	if jti == "" {
		return false, nil
	}
	n, err := b.rdb.Exists(ctx, b.prefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("auth.Blacklist check: %w", err)
	}
	return n > 0, nil
}
