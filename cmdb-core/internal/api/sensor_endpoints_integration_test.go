//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the sensor handlers against a real Postgres so we
// can verify the post-sqlc migration still scopes by tenant correctly.
// The sensors ListSensorsByTenant query joins on assets and the only
// thing protecting cross-tenant leakage is the s.tenant_id predicate —
// easy to break in a future edit, worth pinning with a DB-backed test.
//
// Run with:
//   go test -tags integration -run TestIntegration_Sensors ./internal/api/...

type sensorFixture struct {
	tenantA uuid.UUID
	tenantB uuid.UUID
	sensorA uuid.UUID
	sensorB uuid.UUID
}

// setupSensorFixture creates two isolated tenants each with one sensor.
// Only the tenantA sensor should be visible to tenantA's handler calls.
func setupSensorFixture(t *testing.T, pool *pgxpool.Pool) sensorFixture {
	t.Helper()
	ctx := context.Background()
	fix := sensorFixture{
		tenantA: uuid.New(),
		tenantB: uuid.New(),
		sensorA: uuid.New(),
		sensorB: uuid.New(),
	}

	suffixA := fix.tenantA.String()[:8]
	suffixB := fix.tenantB.String()[:8]

	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES
		   ($1, $2, $3),
		   ($4, $5, $6)`,
		fix.tenantA, "sensor-A-"+suffixA, "sensor-A-"+suffixA,
		fix.tenantB, "sensor-B-"+suffixB, "sensor-B-"+suffixB,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO sensors (id, tenant_id, name, type, polling_interval, enabled, status, created_at, updated_at)
		 VALUES
		   ($1, $2, $3, 'temperature', 30, true, 'offline', now(), now()),
		   ($4, $5, $6, 'humidity',    30, true, 'offline', now(), now())`,
		fix.sensorA, fix.tenantA, "sensor-in-A",
		fix.sensorB, fix.tenantB, "sensor-in-B",
	); err != nil {
		t.Fatalf("insert sensors: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sensors WHERE tenant_id IN ($1, $2)`, fix.tenantA, fix.tenantB)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ($1, $2)`, fix.tenantA, fix.tenantB)
	})
	return fix
}

func newSensorHandlerCtx(t *testing.T, method, target string, tenantID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	c.Set("tenant_id", tenantID.String())
	return c, rec
}

// TestIntegration_ListSensors_TenantScoped asserts that a handler call
// authenticated as tenantA sees exactly one sensor (its own) — the
// tenantB sensor must not leak through the join.
func TestIntegration_ListSensors_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSensorFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSensorHandlerCtx(t, http.MethodGet, "/sensors", fix.tenantA)
	s.ListSensors(c, ListSensorsParams{})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}

	var env struct {
		Data struct {
			Sensors []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"sensors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v — body=%s", err, rec.Body.String())
	}

	if got := len(env.Data.Sensors); got != 1 {
		t.Fatalf("got %d sensors, want 1 (tenantA scope) — body=%s", got, rec.Body.String())
	}
	if env.Data.Sensors[0].ID != fix.sensorA.String() {
		t.Errorf("returned sensor id = %s, want tenantA sensor %s", env.Data.Sensors[0].ID, fix.sensorA)
	}
	if env.Data.Sensors[0].Name != "sensor-in-A" {
		t.Errorf("returned sensor name = %q, want sensor-in-A", env.Data.Sensors[0].Name)
	}
}

// TestIntegration_DeleteSensor_TenantScoped asserts that a delete issued
// while authenticated as tenantA cannot remove a sensor owned by tenantB
// — the sqlc-generated DeleteSensor must require tenant_id match.
func TestIntegration_DeleteSensor_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSensorFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSensorHandlerCtx(t, http.MethodDelete, "/sensors/"+fix.sensorB.String(), fix.tenantA)
	s.DeleteSensor(c, IdPath(fix.sensorB))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — body=%s", rec.Code, rec.Body.String())
	}

	// Verify tenantB's sensor still exists in the DB.
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sensors WHERE id = $1 AND tenant_id = $2`,
		fix.sensorB, fix.tenantB).Scan(&count); err != nil {
		t.Fatalf("count tenantB sensor: %v", err)
	}
	if count != 1 {
		t.Errorf("tenantB sensor was removed by tenantA delete — count=%d, want 1", count)
	}
}

// TestIntegration_SensorHeartbeat_TenantScoped asserts that a heartbeat
// issued while authenticated as tenantA cannot update a sensor owned by
// tenantB. The call "succeeds" at HTTP level (UPDATE with no matching
// rows is not an error in SQL) but the tenantB sensor's last_heartbeat
// must remain NULL.
func TestIntegration_SensorHeartbeat_TenantScoped(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	fix := setupSensorFixture(t, pool)

	s := &APIServer{pool: pool}
	c, rec := newSensorHandlerCtx(t, http.MethodPost, "/sensors/"+fix.sensorB.String()+"/heartbeat", fix.tenantA)
	s.SensorHeartbeat(c, IdPath(fix.sensorB))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body=%s", rec.Code, rec.Body.String())
	}

	var lastHeartbeatNull bool
	if err := pool.QueryRow(context.Background(),
		`SELECT last_heartbeat IS NULL FROM sensors WHERE id = $1`,
		fix.sensorB).Scan(&lastHeartbeatNull); err != nil {
		t.Fatalf("read tenantB sensor heartbeat: %v", err)
	}
	if !lastHeartbeatNull {
		t.Error("tenantB sensor last_heartbeat was updated by tenantA — tenant isolation broken")
	}
}
