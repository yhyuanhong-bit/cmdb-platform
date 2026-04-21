// Package identity holds process-local identity helpers that sit below the
// domain layer. It intentionally does not depend on domain/identity (which
// owns auth_service + role management) to avoid an import cycle: workflows
// depends on this package, and domain/identity depends on workflows
// transitively through its observer hooks.
package identity

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

// SystemUsername is the per-tenant seeded username of the FK-safe sentinel
// user created by migration 000052. Exposed for consistency with the Login
// guard and ListUsers filter that also reference this literal.
const SystemUsername = "system"

// cachedID is the value stored in the SystemUserResolver cache.
type cachedID struct {
	id        uuid.UUID
	expiresAt time.Time
}

// SystemUserResolver returns the UUID of each tenant's username='system'
// user (migration 000052) for FK-safe writes into work_orders.requestor_id,
// work_order_logs.operator_id, and inventory_{tasks,items}.*_by. The
// lookup is cached per-tenant for `ttl` so hot workflow loops don't round
// -trip to Postgres on every iteration.
//
// The resolver is intentionally minimal — no eviction, no size cap, no
// warmup. tenants × sizeof(cachedID) (~48 bytes) is cheap enough that a
// unbounded sync.Map is fine at the fleet sizes we care about. If that
// changes, swap the sync.Map for an LRU later without changing the API.
type SystemUserResolver struct {
	queries *dbgen.Queries
	cache   sync.Map
	ttl     time.Duration
}

// NewSystemUserResolver constructs a resolver. ttl <= 0 falls back to one
// hour, which matches the tenant-trigger seed cadence: even if the DBA
// deletes a tenant's system user, the resolver repairs itself within an
// hour of cache expiry.
func NewSystemUserResolver(queries *dbgen.Queries, ttl time.Duration) *SystemUserResolver {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &SystemUserResolver{queries: queries, ttl: ttl}
}

// SystemUserID returns the system user UUID for the given tenant. The cache
// hit path is lock-free; misses take a single SELECT and race-populate the
// cache (duplicate concurrent inserts are harmless — they resolve to the
// same value). Returns uuid.Nil + error if the row doesn't exist, which
// callers should treat as a *skip this iteration* signal rather than
// silently writing uuid.Nil to a FK column.
func (r *SystemUserResolver) SystemUserID(ctx context.Context, tenantID uuid.UUID) (uuid.UUID, error) {
	if v, ok := r.cache.Load(tenantID); ok {
		c := v.(cachedID)
		if time.Now().Before(c.expiresAt) {
			return c.id, nil
		}
	}

	u, err := r.queries.GetUserByTenantAndUsername(ctx, dbgen.GetUserByTenantAndUsernameParams{
		TenantID: tenantID,
		Username: SystemUsername,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve system user for tenant %s: %w", tenantID, err)
	}

	r.cache.Store(tenantID, cachedID{id: u.ID, expiresAt: time.Now().Add(r.ttl)})
	return u.ID, nil
}

// Invalidate drops the cached entry for a tenant. Useful after admin
// operations that might have rotated the system user row (tests, manual
// DBA intervention). Production code does not need to call this.
func (r *SystemUserResolver) Invalidate(tenantID uuid.UUID) {
	r.cache.Delete(tenantID)
}
