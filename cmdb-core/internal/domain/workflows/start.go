// start.go — unified entry point for every background loop owned by the
// workflows package.
//
// Phase 4.1 (see docs/reports/audit-2026-04-19/REMEDIATION-ROADMAP.md §4.1)
// calls for eventually splitting each subsystem (notifications, sla,
// cleanup, metrics, divergence, autoworkorders, quality) into its own
// subpackage with its own struct. That is a larger, multi-commit
// refactor. This file is the first step of that migration: it collapses
// main.go's 8 scattered Start* calls behind a single StartAll(ctx)
// entry point.
//
// Concrete benefits delivered by this commit:
//
//   - main.go no longer needs to enumerate every background loop by name,
//     so adding a new loop does not require a main.go change.
//   - The StartAll method is the single place that captures the contract
//     between the workflows package and the server lifecycle. Adding
//     cross-cutting guardrails (panic recovery, startup-order
//     dependencies, feature-flag gating) now has one obvious home.
//   - A follow-up commit that splits a subsystem into its own subpackage
//     will wire into StartAll without touching main.go again.
//
// The per-subsystem Start* methods remain exported so tests that exercise
// a single loop in isolation can still call them directly.
package workflows

import "context"

// StartAll launches every background goroutine managed by this
// WorkflowSubscriber. The call is non-blocking: each Start* method
// spawns its own goroutine and returns. SIGTERM propagates through the
// shared ctx and every loop exits within one tick.
//
// Ordering here is alphabetical-by-name to keep the list auditable. None
// of the loops depend on each other at startup, so the order on the wall
// clock does not matter — the sequence below is purely for readers.
//
// Register() is NOT invoked here: main.go still calls it explicitly
// because some subscribe-calls interact with Register's nil-bus fast
// path in ways that are easier to reason about at the call site. A
// future commit can fold it in once the subpackage split lands.
func (w *WorkflowSubscriber) StartAll(ctx context.Context) {
	w.StartAssetVerificationChecker(ctx)
	// Polls pg_inherits every 5m to publish cmdb_audit_partition_count.
	// Paired with the audit-archive CronJob so a missed monthly run
	// shows up as a gauge dip well before writes start bouncing.
	w.StartAuditPartitionSampler(ctx)
	w.StartConflictAndDiscoveryCleanup(ctx)
	// Gated behind CMDB_INTEGRATION_DIVERGENCE_CHECK=1; default off.
	w.StartDivergenceChecker(ctx)
	w.StartMetricsPuller(ctx)
	// Phase 2.11: daily full-tenant quality scan. First tick runs ~30s
	// after startup so deploys produce an observable scan, then every
	// 24h. No-op when WithQualityScanner was not called.
	w.StartQualityScanner(ctx)
	w.StartSessionCleanup(ctx)
	w.StartSLAChecker(ctx)
	w.StartWarrantyChecker(ctx)
	// Daily sweep for webhook_deliveries (30d) and webhook_deliveries_dlq
	// (90d). Emits webhook_retention_deletes_total so a dead cron is
	// observable.
	w.StartWebhookRetention(ctx)
}
