package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// buildComplianceScanResponse — pure transformer, exhaustive table tests
// ---------------------------------------------------------------------------

func ptrStr(s string) *string { return &s }

func TestBuildComplianceScanResponse_EmptyRows(t *testing.T) {
	assetID := uuid.New()
	resp := buildComplianceScanResponse(assetID, nil)

	if uuid.UUID(resp.AssetId) != assetID {
		t.Errorf("asset_id mismatch: got %v want %v", resp.AssetId, assetID)
	}
	if resp.EventCount != 0 {
		t.Errorf("event_count = %d, want 0", resp.EventCount)
	}
	if resp.Events == nil {
		t.Errorf("events should be non-nil empty slice (UI expects [])")
	}
	if len(resp.Events) != 0 {
		t.Errorf("events len = %d, want 0", len(resp.Events))
	}
	if resp.LastScanAt != nil {
		t.Errorf("last_scan_at = %v, want nil for empty input", resp.LastScanAt)
	}
	if resp.LastScanAction != nil || resp.LastScanModule != nil || resp.LastScanSource != nil {
		t.Errorf("last_scan_* fields should be nil when no events")
	}
}

func TestBuildComplianceScanResponse_TopRowDrivesSummary(t *testing.T) {
	assetID := uuid.New()
	opID := uuid.New()
	t1 := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)

	rows := []auditScanRow{
		{
			Action:     "asset.discovered",
			Module:     ptrStr("discovery"),
			Source:     ptrStr("snmp"),
			OperatorID: &opID,
			CreatedAt:  t1,
		},
		{
			Action:    "asset.synced",
			Module:    ptrStr("integration"),
			Source:    ptrStr("zabbix"),
			CreatedAt: t2,
		},
	}

	resp := buildComplianceScanResponse(assetID, rows)

	if resp.EventCount != 2 {
		t.Errorf("event_count = %d, want 2", resp.EventCount)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("events len = %d, want 2", len(resp.Events))
	}
	if resp.LastScanAt == nil || !resp.LastScanAt.Equal(t1) {
		t.Errorf("last_scan_at = %v, want %v (newest = first row)", resp.LastScanAt, t1)
	}
	if resp.LastScanAction == nil || *resp.LastScanAction != "asset.discovered" {
		t.Errorf("last_scan_action mismatch: got %v", resp.LastScanAction)
	}
	if resp.LastScanModule == nil || *resp.LastScanModule != "discovery" {
		t.Errorf("last_scan_module mismatch: got %v", resp.LastScanModule)
	}
	if resp.LastScanSource == nil || *resp.LastScanSource != "snmp" {
		t.Errorf("last_scan_source mismatch: got %v", resp.LastScanSource)
	}

	// Operator id should propagate on the first event only.
	if resp.Events[0].OperatorId == nil || uuid.UUID(*resp.Events[0].OperatorId) != opID {
		t.Errorf("operator_id mismatch: got %v want %v", resp.Events[0].OperatorId, opID)
	}
	if resp.Events[1].OperatorId != nil {
		t.Errorf("second event operator_id should be nil, got %v", resp.Events[1].OperatorId)
	}
}

func TestBuildComplianceScanResponse_NullModuleAndSource(t *testing.T) {
	// audit_events.module / source are nullable in the schema; we should not
	// crash and should propagate the nullness up through the API payload.
	assetID := uuid.New()
	rows := []auditScanRow{
		{
			Action:    "asset.touched",
			Module:    nil,
			Source:    nil,
			CreatedAt: time.Now(),
		},
	}
	resp := buildComplianceScanResponse(assetID, rows)
	if resp.LastScanModule != nil {
		t.Errorf("last_scan_module should be nil, got %v", *resp.LastScanModule)
	}
	if resp.LastScanSource != nil {
		t.Errorf("last_scan_source should be nil, got %v", *resp.LastScanSource)
	}
	if resp.Events[0].Module != nil || resp.Events[0].Source != nil {
		t.Errorf("event module/source should be nil when row is null")
	}
}

// ---------------------------------------------------------------------------
// GetAssetComplianceScan handler — failure paths that don't need a real DB
// ---------------------------------------------------------------------------

// Reuses mockAssetService from impl_assets_test.go in the same package.

func TestGetAssetComplianceScan_AssetNotFound_404(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()

	svc := &mockAssetService{
		getByIDFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*dbgen.Asset, error) {
			return nil, errors.New("not found")
		},
	}
	s := newAssetsTestServer(svc)

	rec := runHandler(t,
		func(c *gin.Context) { s.GetAssetComplianceScan(c, IdPath(assetID)) },
		http.MethodGet, "/assets/"+assetID.String()+"/compliance-scan", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestGetAssetComplianceScan_NoAssetService_500(t *testing.T) {
	// Defensive: the handler nil-guards assetSvc and returns 500. Confirms
	// we never reach the pool with a nil service wired in.
	s := &APIServer{}
	tenantID := uuid.New()
	assetID := uuid.New()

	rec := runHandler(t,
		func(c *gin.Context) { s.GetAssetComplianceScan(c, IdPath(assetID)) },
		http.MethodGet, "/assets/"+assetID.String()+"/compliance-scan", nil, nil,
		map[string]string{"tenant_id": tenantID.String()})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Sanity check: response JSON contract matches what the frontend expects
// ---------------------------------------------------------------------------

func TestBuildComplianceScanResponse_JSONShape(t *testing.T) {
	assetID := uuid.New()
	rows := []auditScanRow{
		{
			Action:    "asset.discovered",
			Module:    ptrStr("discovery"),
			Source:    ptrStr("snmp"),
			CreatedAt: time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	resp := buildComplianceScanResponse(assetID, rows)
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	for _, want := range []string{
		`"asset_id"`,
		`"event_count":1`,
		`"events"`,
		`"last_scan_at"`,
		`"last_scan_action":"asset.discovered"`,
		`"last_scan_module":"discovery"`,
		`"last_scan_source":"snmp"`,
		`"scanned_at"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("response missing %q\nfull body: %s", want, got)
		}
	}
}
