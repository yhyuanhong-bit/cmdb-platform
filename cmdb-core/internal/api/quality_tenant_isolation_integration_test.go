//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// W6.3 — additional sqlc cross-tenant integration coverage for the
// quality surface that impl_quality_tenant_isolation_integration_test.go
// does not yet pin.
//
// Existing impl_quality_tenant_isolation_integration_test.go covers:
//   - GetAssetQualityHistory cross-tenant → 200 + empty rows
//
// Missing (this file fills the gap — quality has more than just the
// scoring history endpoint):
//
//   - FlagQualityIssue : the INSERT carries tenant_id from the caller's
//     ctx, but the request body picks an arbitrary asset_id. A caller
//     in tenantA could plant a flag with `asset_id` pointing at tenantB's
//     asset → the next quality scan would penalise tenantB's accuracy.
//     The application MUST verify the asset belongs to the caller's
//     tenant before INSERT — pinned here.
//
//   - ResolveQualityFlag : the UPDATE has `WHERE id = $1 AND tenant_id = $2`
//     so a cross-tenant resolve must 404. Pinned here so a future sqlc
//     edit dropping tenant_id immediately fails CI.
//
// Run with:
//
//	go test -tags integration -run TestQualityTenantIsolation \
//	  ./internal/api/...

// ---------------------------------------------------------------------------
// 1. FlagQualityIssue — tenantA must NOT be able to plant a flag against
//    tenantB's asset (would corrupt tenantB's quality scoring next pass).
//
// NOTE: this test EXPOSES a real gap if the flag lands. impl_quality.go
// currently builds CreateQualityFlagParams using the caller's tenant_id
// but does NOT verify req.AssetId belongs to that tenant. The DB FK on
// quality_flags.asset_id → assets.id is tenant-agnostic, so a caller in
// tenantA passing tenantB's asset_id WILL succeed at INSERT — just with
// tenant_id=A and asset_id=B (a mismatched row).
//
// The test asserts the desired post-condition: NO flag row should land
// where (quality_flags.tenant_id != assets.tenant_id of the same asset).
// If this test fails today, that's a real bug — report it, do NOT fix.
// ---------------------------------------------------------------------------

func TestQualityTenantIsolation_FlagIssue_ForeignAsset_DoesNotPlantOnVictimTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupQualityIsoFixture(t, pool)
	s := newQualityIsoServer(pool)

	// tenantA caller posts a flag pointing at tenantB's asset.
	body := []byte(fmt.Sprintf(
		`{"asset_id":"%s","reporter_type":"user","severity":"critical","category":"accuracy","message":"PWNED-from-A"}`,
		fix.assetB,
	))
	c, rec := newDepCtx(t, http.MethodPost,
		"/quality/flag-issue",
		fix.tenantA, fix.userA, body)

	s.FlagQualityIssue(c)
	c.Writer.WriteHeaderNow()

	// We do NOT require a specific status here — the contract under
	// audit is the post-condition: there must be NO flag row that
	// punishes tenantB for tenantA's accusation.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*)
		   FROM quality_flags qf
		   JOIN assets a ON a.id = qf.asset_id
		  WHERE qf.asset_id = $1
		    AND a.tenant_id = $2
		    AND qf.tenant_id != a.tenant_id`,
		fix.assetB, fix.tenantB,
	).Scan(&count); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if count > 0 {
		t.Fatalf("REAL BUG (do not fix here): tenantA planted %d cross-tenant quality flags against tenantB's asset (status=%d, body=%s) — quality scan will penalise tenantB's accuracy next pass",
			count, rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 2. ResolveQualityFlag — tenantA must NOT be able to mark tenantB's
// open flag as resolved (denial-of-triage attack: hide a real issue
// before the operator sees it).
// ---------------------------------------------------------------------------

func TestQualityTenantIsolation_ResolveFlag_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupQualityIsoFixture(t, pool)
	s := newQualityIsoServer(pool)

	// Plant an open flag in tenantB so we have something to attack.
	var flagID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO quality_flags
		   (id, tenant_id, asset_id, reporter_type, severity, category, message, status)
		 VALUES (gen_random_uuid(), $1, $2, 'user', 'high', 'accuracy', 'tenantB legit issue', 'open')
		 RETURNING id`,
		fix.tenantB, fix.assetB,
	).Scan(&flagID); err != nil {
		t.Fatalf("plant tenantB flag: %v", err)
	}

	// tenantA caller tries to resolve tenantB's flag.
	body := []byte(`{"status":"rejected","resolution_note":"hijacked by A"}`)
	c, rec := newDepCtx(t, http.MethodPost,
		"/quality/flags/"+flagID.String()+"/resolve",
		fix.tenantA, fix.userA, body)

	s.ResolveQualityFlag(c, IdPath(flagID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant ResolveQualityFlag accepted: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// Flag must still be 'open' on tenantB.
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM quality_flags WHERE id = $1`, flagID,
	).Scan(&status); err != nil {
		t.Fatalf("read flag status: %v", err)
	}
	if status != "open" {
		t.Fatalf("CRITICAL: tenantB flag status mutated to %q by tenantA caller — want 'open'",
			status)
	}
}

// ---------------------------------------------------------------------------
// 3. ResolveQualityFlag — same-tenant control.
// ---------------------------------------------------------------------------

func TestQualityTenantIsolation_ResolveFlag_SameTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupQualityIsoFixture(t, pool)
	s := newQualityIsoServer(pool)

	// Plant an open flag in tenantA so the rightful owner can resolve it.
	var flagID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO quality_flags
		   (id, tenant_id, asset_id, reporter_type, severity, category, message, status)
		 VALUES (gen_random_uuid(), $1, $2, 'user', 'low', 'completeness', 'real tenantA issue', 'open')
		 RETURNING id`,
		fix.tenantA, fix.assetA,
	).Scan(&flagID); err != nil {
		t.Fatalf("plant tenantA flag: %v", err)
	}

	body := []byte(`{"status":"resolved","resolution_note":"fixed"}`)
	c, rec := newDepCtx(t, http.MethodPost,
		"/quality/flags/"+flagID.String()+"/resolve",
		fix.tenantA, fix.userA, body)

	s.ResolveQualityFlag(c, IdPath(flagID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusOK {
		t.Fatalf("same-tenant resolve failed: status=%d body=%s",
			rec.Code, rec.Body.String())
	}

	// Confirm the response carries the new status (round-trip sanity).
	var env struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if env.Data.Status != "resolved" {
		t.Errorf("response.status = %q, want %q — body=%s",
			env.Data.Status, "resolved", rec.Body.String())
	}
}
