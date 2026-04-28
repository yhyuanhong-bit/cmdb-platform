//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/quality"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cross-tenant isolation pin for GetAssetQualityHistory — audit C4
// (2026-04-28). Before v3.3.10 the sqlc query was WHERE asset_id = $1
// with no tenant_id, so any caller could read any tenant's quality
// score history by guessing an asset UUID.
//
// Run with:
//   go test -tags integration -run TestIntegration_QualityHistoryTenantIsolation \
//     ./internal/api/...

type qualityIsoFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	userA   uuid.UUID
	userB   uuid.UUID
	assetA  uuid.UUID
	assetB  uuid.UUID
}

func setupQualityIsoFixture(t *testing.T, pool *pgxpool.Pool) qualityIsoFixture {
	t.Helper()
	ctx := context.Background()
	fix := qualityIsoFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		userA:   uuid.New(),
		userB:   uuid.New(),
		assetA:  uuid.New(),
		assetB:  uuid.New(),
	}
	sufA := fix.tenantA.String()[:8]
	sufB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "qua-A-"+sufA, "qua-A-"+sufA,
		fix.tenantB, "qua-B-"+sufB, "qua-B-"+sufB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash)
		 VALUES ($1, $2, 'qa-uA-'||$3, 'qa UA', 'a-'||$3||'@t', 'x'),
		        ($4, $5, 'qa-uB-'||$6, 'qa UB', 'b-'||$6||'@t', 'x')`,
		fix.userA, fix.tenantA, sufA,
		fix.userB, fix.tenantB, sufB,
	); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, 'asset A', 'server', 'active'),
		        ($4, $5, $6, 'asset B', 'server', 'active')`,
		fix.assetA, fix.tenantA, "qa-A-"+sufA,
		fix.assetB, fix.tenantB, "qa-B-"+sufB,
	); err != nil {
		t.Fatalf("insert assets: %v", err)
	}
	// Plant 3 quality scores per asset so a leak would show non-empty data.
	if _, err := pool.Exec(ctx,
		`INSERT INTO quality_scores (id, tenant_id, asset_id, completeness, accuracy, timeliness, total_score, scan_date)
		 SELECT gen_random_uuid(), $1, $2, 1.0, 1.0, 1.0, 100, now() - (i || ' days')::interval
		 FROM generate_series(0, 2) i`,
		fix.tenantA, fix.assetA,
	); err != nil {
		t.Fatalf("seed quality_scores tenantA: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO quality_scores (id, tenant_id, asset_id, completeness, accuracy, timeliness, total_score, scan_date)
		 SELECT gen_random_uuid(), $1, $2, 0.5, 0.5, 0.5, 50, now() - (i || ' days')::interval
		 FROM generate_series(0, 2) i`,
		fix.tenantB, fix.assetB,
	); err != nil {
		t.Fatalf("seed quality_scores tenantB: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM quality_scores WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM assets WHERE id IN ($1, $2)`, fix.assetA, fix.assetB)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, fix.userA, fix.userB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

func newQualityIsoServer(pool *pgxpool.Pool) *APIServer {
	q := dbgen.New(pool)
	return &APIServer{pool: pool, qualitySvc: quality.NewService(q, pool)}
}

func TestIntegration_QualityHistoryTenantIsolation_ReadCrossTenantBlocked(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupQualityIsoFixture(t, pool)
	s := newQualityIsoServer(pool)

	c, rec := newDepCtx(t, http.MethodGet,
		"/quality/history/"+fix.assetB.String(),
		fix.tenantA, fix.userA, nil)
	s.GetAssetQualityHistory(c, IdPath(fix.assetB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []struct{ ID string `json:"id"` } `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(env.Data) != 0 {
		t.Fatalf("CRITICAL: tenantA leaked %d quality_scores rows from tenantB — body=%s",
			len(env.Data), rec.Body.String())
	}
}

func TestIntegration_QualityHistoryTenantIsolation_OwnTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupQualityIsoFixture(t, pool)
	s := newQualityIsoServer(pool)

	c, rec := newDepCtx(t, http.MethodGet,
		"/quality/history/"+fix.assetA.String(),
		fix.tenantA, fix.userA, nil)
	s.GetAssetQualityHistory(c, IdPath(fix.assetA))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []struct{ ID string `json:"id"` } `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(env.Data) != 3 {
		t.Fatalf("own-tenant len=%d, want 3 — body=%s", len(env.Data), rec.Body.String())
	}
}
