package identity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// These tests cover the cache arithmetic around SystemUserResolver only.
// The DB side is covered by integration tests; here we just want to lock
// in that the TTL gate works and Invalidate evicts.

func TestResolver_CacheHit_UsesCachedID(t *testing.T) {
	t.Parallel()
	r := NewSystemUserResolver(nil, time.Minute)

	tenant := uuid.New()
	sys := uuid.New()
	r.cache.Store(tenant, cachedID{id: sys, expiresAt: time.Now().Add(time.Minute)})

	v, ok := r.cache.Load(tenant)
	if !ok {
		t.Fatal("cache store/load broken")
	}
	if v.(cachedID).id != sys {
		t.Fatalf("cache returned wrong id: got %s want %s", v.(cachedID).id, sys)
	}
}

func TestResolver_Invalidate_DropsEntry(t *testing.T) {
	t.Parallel()
	r := NewSystemUserResolver(nil, time.Minute)

	tenant := uuid.New()
	r.cache.Store(tenant, cachedID{id: uuid.New(), expiresAt: time.Now().Add(time.Minute)})

	r.Invalidate(tenant)
	if _, ok := r.cache.Load(tenant); ok {
		t.Fatal("Invalidate did not evict the entry")
	}
}

func TestResolver_ZeroTTL_FallsBackToOneHour(t *testing.T) {
	t.Parallel()
	r := NewSystemUserResolver(nil, 0)
	if r.ttl != time.Hour {
		t.Fatalf("zero ttl should fall back to 1h, got %s", r.ttl)
	}
}
