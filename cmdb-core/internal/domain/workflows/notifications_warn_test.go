package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestCreateNotification_ReturnsErrorOnPoolFailure proves the new
// createNotification contract: an INSERT failure surfaces to the
// caller instead of being silently dropped. This is the regression
// guard for the 2026-04-19 audit finding that a broken notifications
// table would have been invisible in production.
func TestCreateNotification_ReturnsErrorOnPoolFailure(t *testing.T) {
	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := w.createNotification(ctx, uuid.New(), uuid.New(),
		"test_type", "title", "body", "resource", uuid.New())
	if err == nil {
		t.Fatalf("expected error from createNotification against failing pool, got nil")
	}
}

// TestWarnNotify_WarnsAndCountsOnFailure proves the warnNotify
// wrapper emits both the Warn log line and the counter increment the
// hot-path notification callers rely on. Without this guard, a
// silent regression that reverted warnNotify to fire-and-forget
// would pass the build.
func TestWarnNotify_WarnsAndCountsOnFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	prev := zap.ReplaceGlobals(zap.New(core))
	defer prev()

	const src = "workflows.notifications.test_source"
	before := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(src, telemetry.ReasonNotificationFailed),
	)

	pool := newFailingPool(t)
	defer pool.Close()
	w := &WorkflowSubscriber{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w.warnNotify(ctx, src, uuid.New(), uuid.New(),
		"test_type", "title", "body", "resource", uuid.New())

	after := testutil.ToFloat64(
		telemetry.ErrorsSuppressedTotal.WithLabelValues(src, telemetry.ReasonNotificationFailed),
	)
	if after <= before {
		t.Fatalf("errors_suppressed_total not incremented (before=%v after=%v)", before, after)
	}

	warnCount := 0
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel {
			warnCount++
		}
	}
	if warnCount == 0 {
		t.Fatalf("expected a Warn log line, got 0; entries=%v", logs.All())
	}
}
