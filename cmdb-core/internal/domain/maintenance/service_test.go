package maintenance

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGenerateCode(t *testing.T) {
	code := generateCode()

	// Should start with "WO-"
	if !strings.HasPrefix(code, "WO-") {
		t.Errorf("order code should start with 'WO-', got %q", code)
	}

	// Should be unique (generate two, they should differ)
	code2 := generateCode()
	if code == code2 {
		t.Errorf("generated codes should be unique, got %q twice", code)
	}
}

func TestGenerateCode_Format(t *testing.T) {
	code := generateCode()
	parts := strings.Split(code, "-")
	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts separated by '-', got %q", code)
	}
	if parts[0] != "WO" {
		t.Errorf("first part should be 'WO', got %q", parts[0])
	}
	// Second part should be the year (4 digits)
	if len(parts[1]) != 4 {
		t.Errorf("second part should be a 4-digit year, got %q", parts[1])
	}
}

func TestValidateApproval_SystemBypass(t *testing.T) {
	// uuid.Nil should bypass approval checks
	err := validateApproval(uuid.Nil, pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, nil)
	if err != nil {
		t.Errorf("system operations (uuid.Nil) should bypass approval, got: %v", err)
	}
}

func TestValidateApproval_InsufficientRole(t *testing.T) {
	operatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	requestorID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	roles := []string{"viewer", "editor"}

	err := validateApproval(operatorID, pgtype.UUID{Bytes: requestorID, Valid: true}, roles)
	if err == nil {
		t.Error("expected error for insufficient role")
	}
	if !strings.Contains(err.Error(), "insufficient permissions") {
		t.Errorf("expected 'insufficient permissions' error, got: %v", err)
	}
}

func TestValidateApproval_SelfApproval(t *testing.T) {
	operatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	roles := []string{"super-admin"}

	err := validateApproval(operatorID, pgtype.UUID{Bytes: operatorID, Valid: true}, roles)
	if err == nil {
		t.Error("expected error for self-approval")
	}
	if !strings.Contains(err.Error(), "self-approval") {
		t.Errorf("expected 'self-approval' error, got: %v", err)
	}
}

func TestValidateApproval_Valid(t *testing.T) {
	operatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	requestorID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	roles := []string{"ops-admin"}

	err := validateApproval(operatorID, pgtype.UUID{Bytes: requestorID, Valid: true}, roles)
	if err != nil {
		t.Errorf("expected no error for valid approval, got: %v", err)
	}
}
