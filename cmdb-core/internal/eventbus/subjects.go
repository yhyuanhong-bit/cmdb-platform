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

	// Sync subjects
	SubjectSyncEnvelope = "sync.envelope"

	SubjectAlertRuleCreated = "alert_rule.created"
	SubjectAlertRuleUpdated = "alert_rule.updated"
	SubjectAlertRuleDeleted = "alert_rule.deleted"

	SubjectOrderAnomaly = "maintenance.order_anomaly"
)
