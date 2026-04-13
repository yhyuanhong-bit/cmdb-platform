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

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name    string
		exec    string
		gov     string
		want    string
		wantErr bool
	}{
		{"pending+submitted", ExecPending, GovSubmitted, StatusSubmitted, false},
		{"pending+approved", ExecPending, GovApproved, StatusApproved, false},
		{"pending+rejected", ExecPending, GovRejected, StatusRejected, false},
		{"working+submitted", ExecWorking, GovSubmitted, StatusInProgress, false},
		{"working+approved", ExecWorking, GovApproved, StatusInProgress, false},
		{"working+rejected", ExecWorking, GovRejected, StatusRejected, false},
		{"done+submitted", ExecDone, GovSubmitted, StatusCompleted, false},
		{"done+approved", ExecDone, GovApproved, StatusCompleted, false},
		{"done+rejected", ExecDone, GovRejected, StatusRejected, false},
		{"done+verified", ExecDone, GovVerified, StatusVerified, false},
		{"pending+verified", ExecPending, GovVerified, StatusVerified, false},
		{"working+verified", ExecWorking, GovVerified, StatusVerified, false},
		{"working+in_progress(dirty)", ExecWorking, "in_progress", StatusInProgress, false},
		{"done+completed(dirty)", ExecDone, "completed", StatusCompleted, false},
		{"invalid exec", "bogus", GovSubmitted, "", true},
		{"invalid gov", ExecPending, "bogus", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeriveStatus(tt.exec, tt.gov)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got status=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DeriveStatus(%q, %q) = %q, want %q", tt.exec, tt.gov, got, tt.want)
			}
		})
	}
}

func TestValidateExecTransition(t *testing.T) {
	tests := []struct {
		from, to string
		wantErr  bool
	}{
		{ExecPending, ExecWorking, false},
		{ExecWorking, ExecDone, false},
		{ExecPending, ExecDone, true},
		{ExecDone, ExecPending, true},
		{ExecWorking, ExecPending, true},
		{"bogus", ExecWorking, true},
	}
	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			err := ValidateExecTransition(tt.from, tt.to)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateGovTransition(t *testing.T) {
	tests := []struct {
		from, to string
		wantErr  bool
	}{
		{GovSubmitted, GovApproved, false},
		{GovSubmitted, GovRejected, false},
		{GovApproved, GovVerified, false},
		{GovRejected, GovSubmitted, false},
		{GovSubmitted, GovVerified, true},
		{GovApproved, GovRejected, true},
		{GovVerified, GovApproved, true},
		{"bogus", GovApproved, true},
	}
	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			err := ValidateGovTransition(tt.from, tt.to)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
