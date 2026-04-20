//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Post-sqlc migration coverage for rack_network_connections handlers.
// Unlike user_sessions, this table DOES carry tenant_id, and it's also
// joined with racks.tenant_id at query time. We verify both the JOIN
// and the WHERE guard actually prevent cross-tenant reads and writes.
//
// Run with:
//   go test -tags integration -run TestIntegration_RackNetwork ./internal/api/...

type rackNetFixture struct {
	tenantA   uuid.UUID
	tenantB   uuid.UUID
	locationA uuid.UUID
	locationB uuid.UUID
	rackA     uuid.UUID
	rackB     uuid.UUID
	connA     uuid.UUID
	connB     uuid.UUID
}

// setupRackNetFixture creates two tenants, each with one rack and one
// network connection. The fixture allows cross-tenant reads and
// writes to be tested.
func setupRackNetFixture(t *testing.T, pool *pgxpool.Pool) rackNetFixture {
	t.Helper()
	ctx := context.Background()
	fix := rackNetFixture{
		tenantA:   uuid.New(),
		tenantB:   uuid.New(),
		locationA: uuid.New(),
		locationB: uuid.New(),
		rackA:     uuid.New(),
		rackB:     uuid.New(),
		connA:     uuid.New(),
		connB:     uuid.New(),
	}

	suffixA := fix.tenantA.String()[:8]
	suffixB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		fix.tenantA, "rnc-A-"+suffixA, "rnc-A-"+suffixA,
		fix.tenantB, "rnc-B-"+suffixB, "rnc-B-"+suffixB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO locations (id, tenant_id, name, slug, level)
		 VALUES
		   ($1, $2, $3, $4, 'room'),
		   ($5, $6, $7, $8, 'room')`,
		fix.locationA, fix.tenantA, "loc-A-"+suffixA, "loc-A-"+suffixA,
		fix.locationB, fix.tenantB, "loc-B-"+suffixB, "loc-B-"+suffixB,
	); err != nil {
		t.Fatalf("insert locations: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO racks (id, tenant_id, location_id, name, total_u)
		 VALUES ($1, $2, $3, $4, 42), ($5, $6, $7, $8, 42)`,
		fix.rackA, fix.tenantA, fix.locationA, "rack-A-"+suffixA,
		fix.rackB, fix.tenantB, fix.locationB, "rack-B-"+suffixB,
	); err != nil {
		t.Fatalf("insert racks: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO rack_network_connections (id, rack_id, tenant_id, source_port, external_device, speed, status, vlans, connection_type)
		 VALUES
		   ($1, $2, $3, 'eth0', 'switch-a', '10G', 'UP', '{10,20}', 'network'),
		   ($4, $5, $6, 'eth0', 'switch-b', '10G', 'UP', '{30}',    'network')`,
		fix.connA, fix.rackA, fix.tenantA,
		fix.connB, fix.rackB, fix.tenantB,
	); err != nil {
		t.Fatalf("insert connections: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM rack_network_connections WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM racks WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM locations WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

func newRackHandlerCtx(t *testing.T, method, target string, tenantID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	return c, rec
}

// TestIntegration_ListRackNetworkConnections_TenantScoped asserts that
// a call authenticated as tenantA against tenantB's rack returns an
// empty list — the racks.tenant_id JOIN filter must block it.
func TestIntegration_ListRackNetworkConnections_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRackNetFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newRackHandlerCtx(t, http.MethodGet, "/racks/"+fix.rackB.String()+"/network-connections", fix.tenantA)
	s.ListRackNetworkConnections(c, IdPath(fix.rackB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Connections []struct {
				ID string `json:"id"`
			} `json:"connections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Connections); got != 0 {
		t.Fatalf("tenantA leak: got %d connections for tenantB rack, want 0 — body=%s", got, rec.Body.String())
	}
}

// TestIntegration_ListRackNetworkConnections_OwnRack asserts the
// happy-path: caller sees their own rack's connections with all
// fields populated.
func TestIntegration_ListRackNetworkConnections_OwnRack(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRackNetFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newRackHandlerCtx(t, http.MethodGet, "/racks/"+fix.rackA.String()+"/network-connections", fix.tenantA)
	s.ListRackNetworkConnections(c, IdPath(fix.rackA))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Connections []struct {
				ID     string `json:"id"`
				Port   string `json:"port"`
				Device string `json:"device"`
			} `json:"connections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	if got := len(env.Data.Connections); got != 1 {
		t.Fatalf("got %d connections, want 1 — body=%s", got, rec.Body.String())
	}
	if env.Data.Connections[0].ID != fix.connA.String() {
		t.Errorf("returned conn id = %s, want %s", env.Data.Connections[0].ID, fix.connA)
	}
	if env.Data.Connections[0].Port != "eth0" {
		t.Errorf("returned port = %q, want eth0", env.Data.Connections[0].Port)
	}
}

// TestIntegration_DeleteRackNetworkConnection_TenantScoped asserts
// that tenantA cannot delete a connection belonging to tenantB's rack
// via the sqlc-backed DELETE. The handler must return 404 and the
// tenantB row must remain intact.
func TestIntegration_DeleteRackNetworkConnection_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRackNetFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newRackHandlerCtx(t, http.MethodDelete,
		"/racks/"+fix.rackB.String()+"/network-connections/"+fix.connB.String(), fix.tenantA)
	s.DeleteRackNetworkConnection(c, IdPath(fix.rackB), fix.connB)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM rack_network_connections WHERE id = $1`,
		fix.connB).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("tenantB connection was removed by tenantA delete — count=%d, want 1", count)
	}
}

// TestIntegration_CreateRackNetworkConnection_OwnRack verifies the
// happy-path insert actually lands a row, the handler echoes the new
// id, and the vlans array round-trips through pg's int[] type.
func TestIntegration_CreateRackNetworkConnection_OwnRack(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupRackNetFixture(t, pool)

	s := &APIServer{pool: pool}
	body := map[string]any{
		"source_port":     "eth99",
		"external_device": "switch-99",
		"speed":           "40G",
		"status":          "UP",
		"vlans":           []int{100, 200, 300},
		"connection_type": "network",
	}
	payload, _ := json.Marshal(body)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPost,
		"/racks/"+fix.rackA.String()+"/network-connections", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("tenant_id", fix.tenantA.String())

	s.CreateRackNetworkConnection(c, IdPath(fix.rackA))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 — body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}
	newID, err := uuid.Parse(env.Data.ID)
	if err != nil {
		t.Fatalf("parse returned id: %v (got %q)", err, env.Data.ID)
	}

	var gotVlans []int32
	if err := pool.QueryRow(context.Background(),
		`SELECT vlans FROM rack_network_connections WHERE id = $1 AND tenant_id = $2`,
		newID, fix.tenantA).Scan(&gotVlans); err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := []int32{100, 200, 300}
	if len(gotVlans) != len(want) {
		t.Fatalf("vlans length = %d, want %d", len(gotVlans), len(want))
	}
	for i := range want {
		if gotVlans[i] != want[i] {
			t.Errorf("vlans[%d] = %d, want %d", i, gotVlans[i], want[i])
		}
	}
}
