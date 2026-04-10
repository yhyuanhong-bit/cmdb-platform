package maintenance

import "testing"

func TestValidateTransition(t *testing.T) {
	validTransitions := []struct {
		from, to string
	}{
		{StatusSubmitted, StatusApproved},
		{StatusSubmitted, StatusRejected},
		{StatusApproved, StatusInProgress},
		{StatusInProgress, StatusCompleted},
		{StatusCompleted, StatusVerified},
		{StatusRejected, StatusSubmitted},
	}

	for _, tt := range validTransitions {
		if err := ValidateTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateTransition(%q, %q) should be valid, got error: %v", tt.from, tt.to, err)
		}
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	invalidTransitions := []struct {
		from, to string
	}{
		{StatusSubmitted, StatusInProgress},
		{StatusSubmitted, StatusCompleted},
		{StatusApproved, StatusRejected},
		{StatusApproved, StatusCompleted},
		{StatusInProgress, StatusSubmitted},
		{StatusCompleted, StatusInProgress},
		{StatusVerified, StatusSubmitted},
	}

	for _, tt := range invalidTransitions {
		if err := ValidateTransition(tt.from, tt.to); err == nil {
			t.Errorf("ValidateTransition(%q, %q) should be invalid, but got no error", tt.from, tt.to)
		}
	}
}

func TestValidateTransition_UnknownStatus(t *testing.T) {
	if err := ValidateTransition("nonexistent", "submitted"); err == nil {
		t.Error("expected error for unknown status")
	}
}

func TestRequiresApproval(t *testing.T) {
	if !RequiresApproval(StatusApproved) {
		t.Error("approved should require approval")
	}
	if !RequiresApproval(StatusRejected) {
		t.Error("rejected should require approval")
	}
	if RequiresApproval(StatusInProgress) {
		t.Error("in_progress should not require approval")
	}
	if RequiresApproval(StatusCompleted) {
		t.Error("completed should not require approval")
	}
}
