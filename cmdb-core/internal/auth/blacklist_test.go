package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeRedis is a minimal in-memory stand-in for *redis.Client that satisfies
// the subset of the API exercised by Blacklist (Set + Exists with TTL).
// Miniredis would give us the same behaviour but pulling a new dep just for
// two methods is not worth it.
type fakeRedis struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]fakeEntry
	setErr  error
	existsErr error
}

type fakeEntry struct {
	value     string
	expiresAt time.Time // zero = no expiry
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		now:     time.Now,
		entries: make(map[string]fakeEntry),
	}
}

func (f *fakeRedis) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	base := f.now()
	f.now = func() time.Time { return base.Add(d) }
}

func (f *fakeRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "set", key)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		cmd.SetErr(f.setErr)
		return cmd
	}
	valStr, _ := value.(string)
	entry := fakeEntry{value: valStr}
	if expiration > 0 {
		entry.expiresAt = f.now().Add(expiration)
	}
	f.entries[key] = entry
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, "exists")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.existsErr != nil {
		cmd.SetErr(f.existsErr)
		return cmd
	}
	var count int64
	for _, k := range keys {
		e, ok := f.entries[k]
		if !ok {
			continue
		}
		if !e.expiresAt.IsZero() && !f.now().Before(e.expiresAt) {
			delete(f.entries, k)
			continue
		}
		count++
	}
	cmd.SetVal(count)
	return cmd
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBlacklist_RevokeAndCheck(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRedis()
	bl := newBlacklistWithDoer(fr, "")

	jti := "jti-abc"
	expiresAt := fr.now().Add(10 * time.Second)

	if err := bl.Revoke(ctx, jti, expiresAt); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	revoked, err := bl.IsRevoked(ctx, jti)
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Error("expected jti to be revoked immediately after Revoke")
	}

	// Fast-forward past the TTL; the entry should self-expire.
	fr.advance(11 * time.Second)
	revoked, err = bl.IsRevoked(ctx, jti)
	if err != nil {
		t.Fatalf("IsRevoked after expiry: %v", err)
	}
	if revoked {
		t.Error("expected jti to be cleared after TTL")
	}
}

func TestBlacklist_EmptyJTI(t *testing.T) {
	ctx := context.Background()
	bl := newBlacklistWithDoer(newFakeRedis(), "")

	revoked, err := bl.IsRevoked(ctx, "")
	if err != nil {
		t.Errorf("IsRevoked(\"\") err = %v, want nil", err)
	}
	if revoked {
		t.Error("IsRevoked(\"\") should be false")
	}

	if err := bl.Revoke(ctx, "", time.Now().Add(time.Hour)); err == nil {
		t.Error("Revoke(\"\") should error")
	}
}

func TestBlacklist_ExpiredTokenSkipsSet(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRedis()
	bl := newBlacklistWithDoer(fr, "")

	// expiresAt in the past -> Revoke is a no-op.
	if err := bl.Revoke(ctx, "jti-stale", fr.now().Add(-time.Hour)); err != nil {
		t.Fatalf("Revoke of already-expired token should not error, got %v", err)
	}

	revoked, err := bl.IsRevoked(ctx, "jti-stale")
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if revoked {
		t.Error("expired-token revoke should not persist anything")
	}
	// And nothing should have been written to the backing store.
	if len(fr.entries) != 0 {
		t.Errorf("expected no writes for expired token, got %d entries", len(fr.entries))
	}
}

func TestBlacklist_PrefixIsolation(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRedis()
	bl := newBlacklistWithDoer(fr, "jti_blacklist:")

	if err := bl.Revoke(ctx, "abc", fr.now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Verify the key lives in the prefixed namespace.
	if _, ok := fr.entries["jti_blacklist:abc"]; !ok {
		t.Error("expected key under jti_blacklist: prefix")
	}
}

func TestBlacklist_NilRedisClient(t *testing.T) {
	ctx := context.Background()
	bl := &Blacklist{rdb: nil, prefix: defaultPrefix}

	if err := bl.Revoke(ctx, "abc", time.Now().Add(time.Hour)); err == nil {
		t.Error("Revoke on unconfigured blacklist should error")
	}
	revoked, err := bl.IsRevoked(ctx, "abc")
	if err != nil {
		t.Errorf("IsRevoked on unconfigured blacklist should not error: %v", err)
	}
	if revoked {
		t.Error("unconfigured blacklist should never report revoked")
	}
}

func TestBlacklist_RedisErrorPropagated(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRedis()
	fr.existsErr = errors.New("redis: connection refused")
	bl := newBlacklistWithDoer(fr, "")

	_, err := bl.IsRevoked(ctx, "abc")
	if err == nil {
		t.Error("expected error to be surfaced so middleware can log/fail-open")
	}
}

func TestBlacklist_NewBlacklistUsesRealClient(t *testing.T) {
	// NewBlacklist takes a *redis.Client to keep the public API stable.
	// We can't exercise the real Redis here, but we can ensure the
	// constructor returns a non-nil Blacklist configured with the default
	// prefix.
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer rc.Close()
	bl := NewBlacklist(rc)
	if bl == nil {
		t.Fatal("NewBlacklist returned nil")
	}
	if bl.prefix != defaultPrefix {
		t.Errorf("prefix = %q, want %q", bl.prefix, defaultPrefix)
	}
}
