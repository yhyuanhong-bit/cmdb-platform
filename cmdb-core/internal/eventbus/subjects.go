package eventbus

// Subject constants for all domain events in the CMDB platform.
const (
	SubjectAssetCreated         = "asset.created"
	SubjectAssetUpdated         = "asset.updated"
	SubjectAssetStatusChanged   = "asset.status_changed"
	SubjectAssetDeleted         = "asset.deleted"
	SubjectLocationCreated      = "location.created"
	SubjectLocationUpdated      = "location.updated"
	SubjectLocationDeleted      = "location.deleted"
	SubjectRackCreated          = "rack.created"
	SubjectRackUpdated          = "rack.updated"
	SubjectRackDeleted          = "rack.deleted"
	SubjectRackOccupancyChanged = "rack.occupancy_changed"

	SubjectOrderCreated      = "maintenance.order_created"
	SubjectOrderUpdated      = "maintenance.order_updated"
	SubjectOrderTransitioned = "maintenance.order_transitioned"

	SubjectInventoryTaskCreated   = "inventory.task_created"
	SubjectInventoryTaskCompleted = "inventory.task_completed"
	SubjectInventoryItemScanned   = "inventory.item_scanned"
	SubjectInventoryItemCreated   = "inventory.item_created"
	SubjectInventoryItemUpdated   = "inventory.item_updated"

	SubjectAlertFired    = "alert.fired"
	SubjectAlertResolved = "alert.resolved"

	SubjectImportCompleted = "import.completed"
	SubjectConflictCreated = "import.conflict_created"

	SubjectPredictionCreated = "prediction.created"

	SubjectAuditRecorded = "audit.recorded"

	SubjectNotificationCreated = "notification.created"

	SubjectAlertRuleCreated = "alert_rule.created"
	SubjectAlertRuleUpdated = "alert_rule.updated"
	SubjectAlertRuleDeleted = "alert_rule.deleted"

	SubjectOrderAnomaly = "maintenance.order_anomaly"

	SubjectAssetLocationChanged = "asset.location_changed"

	SubjectScanDifferencesDetected = "scan.differences_detected"

	SubjectBMCDefaultPassword = "scan.bmc_default_password"

	// SubjectWebhookDisabled is published when the circuit breaker trips a
	// webhook subscription. Payload: {"webhook_id","tenant_id","reason",
	// "consecutive_failures"}. Ops-admin notification listeners subscribe
	// so a human knows to fix the receiver and re-enable.
	SubjectWebhookDisabled = "webhook.disabled"

	// Business Service entity (Wave 2). Emitted by domain/service on
	// CRUD; consumed by Wave 6 incident aggregation (service tier drives
	// severity) and Wave 8 rack heatmap (filtering by service).
	SubjectServiceCreated = "service.created"
	SubjectServiceUpdated = "service.updated"
	SubjectServiceDeleted = "service.deleted"
)
