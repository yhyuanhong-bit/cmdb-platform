package sync

import (
	"testing"
)

func TestIsFromCentral(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"central", true},
		{"edge-taipei", false},
		{"edge-kaohsiung", false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			env := SyncEnvelope{Source: tt.source}
			if got := isFromCentral(env); got != tt.want {
				t.Errorf("isFromCentral(source=%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestDeriveStatusSQL(t *testing.T) {
	tests := []struct {
		exec, gov string
		want      string
	}{
		{"pending", "submitted", "submitted"},
		{"pending", "approved", "approved"},
		{"pending", "rejected", "rejected"},
		{"working", "submitted", "in_progress"},
		{"working", "approved", "in_progress"},
		{"working", "rejected", "rejected"},
		{"done", "submitted", "completed"},
		{"done", "approved", "completed"},
		{"done", "rejected", "rejected"},
		{"done", "verified", "verified"},
		{"pending", "verified", "verified"},
		{"working", "verified", "verified"},
		// Dirty backfill tolerance
		{"working", "in_progress", "in_progress"},
		{"done", "completed", "completed"},
	}
	for _, tt := range tests {
		t.Run(tt.exec+"+"+tt.gov, func(t *testing.T) {
			got := deriveStatusSQL(tt.exec, tt.gov)
			if got != tt.want {
				t.Errorf("deriveStatusSQL(%q, %q) = %q, want %q", tt.exec, tt.gov, got, tt.want)
			}
		})
	}
}
