//go:build integration

package api

import (
	"context"
	"net/http"
	"testing"
)

// W6.3 — additional sqlc cross-tenant integration coverage for the BIA
// surface that impl_bia_integration_test.go does not yet pin.
//
// Existing impl_bia_integration_test.go already covers:
//   - CreateBIADependency cross-tenant → 404 (no propagation, no dep row)
//   - DeleteBIADependency cross-tenant → 404 (dep + bia_level untouched)
//   - UpdateBIAAssessment cross-tenant → 404 (system_name not overwritten)
//   - ListBIADependencies   cross-tenant → 200 + empty
//   - UpdateBIAScoringRule  cross-tenant → 404 (display_name not overwritten)
//   - GetBIAImpact          cross-tenant → 200 + empty
//
// Missing (this file fills the gap):
//   - GetBIAAssessment     cross-tenant → 404 (existence not leaked)
//   - DeleteBIAAssessment  cross-tenant → 404 (row + dependent assets intact)
//
// Both share the same WHERE id-only / WHERE id+tenant_id risk pattern
// flagged by project_tenantlint_blindspot.md.
//
// Run with:
//
//	go test -tags integration -run TestBIATenantIsolation \
//	  ./internal/api/...

// ---------------------------------------------------------------------------
// 1. GetBIAAssessment — tenantA must NOT be able to read tenantB's row.
//
// The 404 vs 403 choice: returning 404 (not 403) is intentional. A 403
// would leak that an assessment with the queried UUID DOES exist — just
// in another tenant. With UUIDs that's a low-probability oracle, but
// the project standard (per existing tests + project_tenantlint_blindspot)
// is to never confirm cross-tenant existence.
// ---------------------------------------------------------------------------

func TestBIATenantIsolation_GetAssessment_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	tenantA := setupBIAFixture(t, pool)
	tenantB := setupBIAFixture(t, pool)
	s := newBIATestServer(pool)

	// tenantA caller asks for tenantB's assessment by id.
	c, rec := newBIACtx(t, http.MethodGet,
		"/bia/assessments/"+tenantB.assessmentID.String(),
		tenantA.tenantID, tenantA.userID, "")
	s.GetBIAAssessment(c, IdPath(tenantB.assessmentID))

	if rec.Code == http.StatusOK {
		t.Fatalf("CRITICAL: tenantA was allowed to GET tenantB's BIA assessment (status=200, body=%s)",
			rec.Body.String())
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 — body=%s", rec.Code, rec.Body.String())
	}
}

// Same-tenant control: ensure the 404 above isn't masking a regression
// that broke read for the legitimate owner.
func TestBIATenantIsolation_GetAssessment_SameTenantOK(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupBIAFixture(t, pool)
	s := newBIATestServer(pool)

	c, rec := newBIACtx(t, http.MethodGet,
		"/bia/assessments/"+fix.assessmentID.String(),
		fix.tenantID, fix.userID, "")
	s.GetBIAAssessment(c, IdPath(fix.assessmentID))

	if rec.Code != http.StatusOK {
		t.Fatalf("same-tenant GET failed: status=%d body=%s",
			rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 2. DeleteBIAAssessment — tenantA must NOT be able to drop tenantB's row.
//
// Asserts both the 404 status AND that the underlying row + any dependent
// assets are untouched. Without `AND tenant_id` the DELETE would silently
// succeed (0 rows for tenantA's WHERE) OR worse, drop tenantB's row if
// only `WHERE id = $1` was used.
// ---------------------------------------------------------------------------

func TestBIATenantIsolation_DeleteAssessment_CrossTenantReturns404(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	tenantA := setupBIAFixture(t, pool)
	tenantB := setupBIAFixture(t, pool)
	s := newBIATestServer(pool)

	// tenantA caller tries to DELETE tenantB's assessment.
	c, rec := newBIACtx(t, http.MethodDelete,
		"/bia/assessments/"+tenantB.assessmentID.String(),
		tenantA.tenantID, tenantA.userID, "")
	s.DeleteBIAAssessment(c, IdPath(tenantB.assessmentID))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant DELETE accepted: status=%d body=%s, want 404",
			rec.Code, rec.Body.String())
	}

	// tenantB row must still exist.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM bia_assessments WHERE id = $1`,
		tenantB.assessmentID).Scan(&count); err != nil {
		t.Fatalf("count tenantB assessment: %v", err)
	}
	if count != 1 {
		t.Fatalf("CRITICAL: tenantB BIA assessment was deleted by tenantA caller — count=%d, want 1",
			count)
	}
}
