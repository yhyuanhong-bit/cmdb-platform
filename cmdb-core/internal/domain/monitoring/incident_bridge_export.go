//go:build integration

package monitoring

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Wave 5.4 test-only entry point. The bridge's real method onAlertEmitted
// is package-private because production callers only reach it via emit().
// Integration tests live in `package api_test`, so we need an exported
// shim — but we never want this in production binaries. The
// `//go:build integration` tag keeps it out of regular builds while
// still letting `go test -tags integration ./...` find it.

// BridgeForTest is the integration-test handle. Construct via
// NewBridgeForTest — the inner bridge stays unexported.
type BridgeForTest struct {
	inner *incidentBridge
}

// NewBridgeForTest wires a bridge to the given pool with a no-op logger
// and a time.Now clock. The DB's own now() drives dedupe-window
// behaviour, so injecting a fake clock here would be misleading.
func NewBridgeForTest(pool *pgxpool.Pool) *BridgeForTest {
	return &BridgeForTest{
		inner: &incidentBridge{
			pool:   pool,
			logger: zap.NewNop(),
			now:    time.Now,
		},
	}
}

// OnAlertEmittedForTest mirrors the production emit() → onAlertEmitted
// call site. Tests call this directly to drive the bridge without
// involving the rule evaluator or the metrics aggregation path.
func (b *BridgeForTest) OnAlertEmittedForTest(
	ctx context.Context,
	tenantID, alertID, assetID uuid.UUID,
	severity, status, message string,
) {
	b.inner.onAlertEmitted(ctx, tenantID, alertID, assetID, severity, status, message)
}
