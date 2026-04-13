package eventbus

import (
	"strings"
	"testing"
)

func TestSubjectConstants(t *testing.T) {
	subjects := []struct {
		name  string
		value string
	}{
		{"SubjectAssetCreated", SubjectAssetCreated},
		{"SubjectAssetUpdated", SubjectAssetUpdated},
		{"SubjectAssetStatusChanged", SubjectAssetStatusChanged},
		{"SubjectAssetDeleted", SubjectAssetDeleted},
		{"SubjectLocationCreated", SubjectLocationCreated},
		{"SubjectLocationUpdated", SubjectLocationUpdated},
		{"SubjectLocationDeleted", SubjectLocationDeleted},
		{"SubjectRackCreated", SubjectRackCreated},
		{"SubjectRackUpdated", SubjectRackUpdated},
		{"SubjectRackDeleted", SubjectRackDeleted},
		{"SubjectRackOccupancyChanged", SubjectRackOccupancyChanged},
		{"SubjectOrderCreated", SubjectOrderCreated},
		{"SubjectOrderUpdated", SubjectOrderUpdated},
		{"SubjectOrderTransitioned", SubjectOrderTransitioned},
		{"SubjectInventoryTaskCreated", SubjectInventoryTaskCreated},
		{"SubjectInventoryTaskCompleted", SubjectInventoryTaskCompleted},
		{"SubjectInventoryItemScanned", SubjectInventoryItemScanned},
		{"SubjectAlertFired", SubjectAlertFired},
		{"SubjectAlertResolved", SubjectAlertResolved},
		{"SubjectImportCompleted", SubjectImportCompleted},
		{"SubjectConflictCreated", SubjectConflictCreated},
		{"SubjectPredictionCreated", SubjectPredictionCreated},
		{"SubjectAuditRecorded", SubjectAuditRecorded},
		{"SubjectNotificationCreated", SubjectNotificationCreated},
	}

	for _, s := range subjects {
		t.Run(s.name, func(t *testing.T) {
			if s.value == "" {
				t.Errorf("%s should not be empty", s.name)
			}
			if !strings.Contains(s.value, ".") {
				t.Errorf("%s = %q should contain a dot separator", s.name, s.value)
			}
		})
	}
}

func TestSubjectNamingConvention(t *testing.T) {
	// All subjects should follow "domain.action" pattern
	tests := []struct {
		name           string
		subject        string
		expectedDomain string
	}{
		{"asset created", SubjectAssetCreated, "asset"},
		{"asset updated", SubjectAssetUpdated, "asset"},
		{"asset deleted", SubjectAssetDeleted, "asset"},
		{"location created", SubjectLocationCreated, "location"},
		{"rack created", SubjectRackCreated, "rack"},
		{"rack occupancy", SubjectRackOccupancyChanged, "rack"},
		{"order created", SubjectOrderCreated, "maintenance"},
		{"order transitioned", SubjectOrderTransitioned, "maintenance"},
		{"alert fired", SubjectAlertFired, "alert"},
		{"alert resolved", SubjectAlertResolved, "alert"},
		{"inventory task created", SubjectInventoryTaskCreated, "inventory"},
		{"import completed", SubjectImportCompleted, "import"},
		{"prediction created", SubjectPredictionCreated, "prediction"},
		{"audit recorded", SubjectAuditRecorded, "audit"},
		{"notification created", SubjectNotificationCreated, "notification"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.SplitN(tt.subject, ".", 2)
			if len(parts) < 2 {
				t.Fatalf("subject %q missing dot separator", tt.subject)
			}
			if parts[0] != tt.expectedDomain {
				t.Errorf("subject %q should start with %q, got %q", tt.subject, tt.expectedDomain, parts[0])
			}
		})
	}
}

func TestSubjectUniqueness(t *testing.T) {
	subjects := []string{
		SubjectAssetCreated,
		SubjectAssetUpdated,
		SubjectAssetStatusChanged,
		SubjectAssetDeleted,
		SubjectLocationCreated,
		SubjectLocationUpdated,
		SubjectLocationDeleted,
		SubjectRackCreated,
		SubjectRackUpdated,
		SubjectRackDeleted,
		SubjectRackOccupancyChanged,
		SubjectOrderCreated,
		SubjectOrderUpdated,
		SubjectOrderTransitioned,
		SubjectInventoryTaskCreated,
		SubjectInventoryTaskCompleted,
		SubjectInventoryItemScanned,
		SubjectAlertFired,
		SubjectAlertResolved,
		SubjectImportCompleted,
		SubjectConflictCreated,
		SubjectPredictionCreated,
		SubjectAuditRecorded,
		SubjectNotificationCreated,
	}

	seen := make(map[string]bool)
	for _, s := range subjects {
		if seen[s] {
			t.Errorf("duplicate subject constant: %q", s)
		}
		seen[s] = true
	}
}
