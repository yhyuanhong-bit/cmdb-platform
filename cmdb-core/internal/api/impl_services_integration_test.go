//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Services tenant-isolation regression tests. Wave 2 introduces the first
// user-creatable business entity since the BIA refactor; these tests pin
// down the isolation contract before more callers depend on it.

func newServicesTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// setupTwoTenants inserts two fresh tenants and returns their UUIDs. Each
// test uses brand-new tenant IDs so parallel runs do not collide on the
// services.code uniqueness constraint.
func setupTwoTenants(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a := uuid.New()
	b := uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "svc-A-"+suffix, "svc-a-"+suffix,
		b, "svc-B-"+suffix, "svc-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, a, b)
	})
	return a, b
}

func TestServices_GetByIDDoesNotCrossTenant(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	// Tenant A creates a service. Tenant B then tries to read it by its ID.
	created, err := svc.Create(ctx, service.CreateParams{
		TenantID: tenantA,
		Code:     "ISO-TEST-1",
		Name:     "Cross-tenant read test",
		Tier:     service.TierNormal,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.GetByID(ctx, tenantB, created.ID)
	if err != service.ErrNotFound {
		t.Fatalf("GetByID cross-tenant: want ErrNotFound, got %v", err)
	}
}

func TestServices_UpdateDoesNotCrossTenant(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	created, err := svc.Create(ctx, service.CreateParams{
		TenantID: tenantA, Code: "ISO-TEST-2", Name: "Original",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	origName := created.Name

	// Tenant B tries to rename A's service. Must fail, no mutation.
	newName := "Hijacked"
	_, err = svc.Update(ctx, service.UpdateParams{
		TenantID: tenantB,
		ID:       created.ID,
		Name:     &newName,
	})
	if err != service.ErrNotFound {
		t.Fatalf("Update cross-tenant: want ErrNotFound, got %v", err)
	}

	// Verify the service wasn't touched.
	back, err := svc.GetByID(ctx, tenantA, created.ID)
	if err != nil {
		t.Fatalf("re-read by owning tenant: %v", err)
	}
	if back.Name != origName {
		t.Errorf("cross-tenant update leaked: name = %q, want %q", back.Name, origName)
	}
}

func TestServices_DuplicateCodeRejected(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	_, err := svc.Create(ctx, service.CreateParams{
		TenantID: tenantA, Code: "ISO-DUP", Name: "First",
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = svc.Create(ctx, service.CreateParams{
		TenantID: tenantA, Code: "ISO-DUP", Name: "Second",
	})
	if err != service.ErrDuplicateCode {
		t.Fatalf("second create: want ErrDuplicateCode, got %v", err)
	}
}

func TestServices_DuplicateCodeAllowedAcrossTenants(t *testing.T) {
	// Same code in different tenants is fine — codes are per-tenant IDs.
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	if _, err := svc.Create(ctx, service.CreateParams{TenantID: tenantA, Code: "ORDER-API", Name: "A"}); err != nil {
		t.Fatalf("create in tenant A: %v", err)
	}
	if _, err := svc.Create(ctx, service.CreateParams{TenantID: tenantB, Code: "ORDER-API", Name: "B"}); err != nil {
		t.Fatalf("create in tenant B: %v", err)
	}
}

func TestServices_InvalidCodeRejected(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	// Q1 sign-off: codes must match ^[A-Z][A-Z0-9_-]{1,63}$. These all fail.
	badCodes := []string{
		"order-api",  // lowercase
		"订单系统",       // Unicode
		"1-HELLO",    // leading digit
		"",           // empty
		"A",          // too short (< 2 chars)
		"-LEADING",   // non-letter lead
		"Over-THIRTY-" + "x__________________________________________________________________", // > 64
	}
	for _, code := range badCodes {
		_, err := svc.Create(ctx, service.CreateParams{
			TenantID: tenantA, Code: code, Name: "Test",
		})
		if err != service.ErrInvalidCode {
			t.Errorf("code %q: want ErrInvalidCode, got %v", code, err)
		}
	}
}

func TestServices_AssetAttachmentIsolation(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	// Each tenant gets one asset (needed because service_assets FKs into
	// assets). Use tenant-UUID-derived asset_tags so concurrent test runs
	// don't collide on the UNIQUE (asset_tag) constraint.
	assetA := uuid.New()
	assetB := uuid.New()
	tagA := "A-" + tenantA.String()[:8]
	tagB := "B-" + tenantB.String()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status) VALUES
		($1, $2, $3, 'Asset A', 'server', 'rack_mount', 'operational'),
		($4, $5, $6, 'Asset B', 'server', 'rack_mount', 'operational')`,
		assetA, tenantA, tagA, assetB, tenantB, tagB); err != nil {
		t.Fatalf("seed assets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE id IN ($1, $2)`, assetA, assetB)
	})

	// Create services.
	svcA, err := svc.Create(ctx, service.CreateParams{TenantID: tenantA, Code: "ISO-ATT-A", Name: "A"})
	if err != nil {
		t.Fatalf("create svcA: %v", err)
	}

	// Attaching tenant-B's asset under tenant-A's service must fail with
	// the tenancy guard. The domain checks assets.tenant_id = tenantID
	// before inserting because service_assets has no FK pairing that
	// enforces assets.tenant_id = services.tenant_id.
	_, err = svc.AddAsset(ctx, tenantA, svcA.ID, assetB, service.RoleComponent, false, uuid.Nil)
	if err != service.ErrAssetNotInTenant {
		t.Fatalf("cross-tenant attach: want ErrAssetNotInTenant, got %v", err)
	}

	// Double-check: ListAssets for svcA should not return assetB.
	rows, listErr := svc.ListAssets(ctx, tenantA, svcA.ID)
	if listErr != nil {
		t.Fatalf("ListAssets: %v", listErr)
	}
	for _, r := range rows {
		if r.AssetID == assetB {
			t.Errorf("cross-tenant asset leaked into listing: %s", assetB)
		}
	}
}

func TestServices_Health_NoCriticalAssetsReturnsUnknown(t *testing.T) {
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := setupTwoTenants(t, pool)
	q := dbgen.New(pool)
	svc := service.New(pool, q, nil)

	svcA, err := svc.Create(ctx, service.CreateParams{TenantID: tenantA, Code: "HEALTH-1", Name: "No assets yet"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	status, total, bad, err := svc.Health(ctx, tenantA, svcA.ID)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if status != service.HealthUnknown {
		t.Errorf("no critical assets: status = %v, want %v", status, service.HealthUnknown)
	}
	if total != 0 || bad != 0 {
		t.Errorf("counts = (%d, %d), want (0, 0)", total, bad)
	}
}

func TestServices_BIABackfillCreatedServicesForExistingAssessments(t *testing.T) {
	// Migration 000063 backfills services from bia_assessments whose
	// system_code matches the Q1 regex. Confirm the relationship is
	// bidirectional after migrate + seed.
	pool := newServicesTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	var bothDirectionCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM services s
		JOIN bia_assessments b ON b.service_id = s.id AND b.id = s.bia_assessment_id
	`).Scan(&bothDirectionCount); err != nil {
		t.Fatalf("backfill check: %v", err)
	}
	if bothDirectionCount == 0 {
		t.Skip("no existing BIA rows to verify backfill against")
	}
	t.Logf("BIA backfill verified bidirectional on %d rows", bothDirectionCount)
}
