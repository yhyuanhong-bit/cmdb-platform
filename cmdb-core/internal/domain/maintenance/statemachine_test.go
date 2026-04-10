package maintenance

import "testing"

func TestValidateTransition(t *testing.T) {
	validTransitions := []struct {
		from, to string
	}{
		{"draft", "pending"},
		{"pending", "approved"},
		{"pending", "rejected"},
		{"approved", "in_progress"},
		{"in_progress", "completed"},
		{"completed", "closed"},
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
		{"draft", "completed"},
		{"draft", "in_progress"},
		{"pending", "in_progress"},
		{"approved", "rejected"},
		{"in_progress", "draft"},
		{"completed", "in_progress"},
		{"closed", "draft"},
	}

	for _, tt := range invalidTransitions {
		if err := ValidateTransition(tt.from, tt.to); err == nil {
			t.Errorf("ValidateTransition(%q, %q) should be invalid, but got no error", tt.from, tt.to)
		}
	}
}

func TestValidateTransition_UnknownStatus(t *testing.T) {
	if err := ValidateTransition("nonexistent", "draft"); err == nil {
		t.Error("expected error for unknown status")
	}
}
