package mcp

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"one hour", "1h", 1 * time.Hour},
		{"six hours", "6h", 6 * time.Hour},
		{"twenty four hours", "24h", 24 * time.Hour},
		{"seven days", "7d", 7 * 24 * time.Hour},
		{"thirty days", "30d", 30 * 24 * time.Hour},
		{"one day", "1d", 24 * time.Hour},
		{"empty defaults to 24h", "", 24 * time.Hour},
		{"invalid falls back to 24h", "invalid", 24 * time.Hour},
		{"negative falls back to 24h", "-1h", 24 * time.Hour},
		{"thirty minutes", "30m", 30 * time.Minute},
		{"zero days falls back", "0d", 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestOptText(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		key       string
		wantValid bool
		wantStr   string
	}{
		{"present value", map[string]any{"type": "server"}, "type", true, "server"},
		{"missing key", map[string]any{}, "type", false, ""},
		{"nil value", map[string]any{"type": nil}, "type", false, ""},
		{"empty string", map[string]any{"type": ""}, "type", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optText(tt.args, tt.key)
			if got.Valid != tt.wantValid {
				t.Errorf("optText valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if got.Valid && got.String != tt.wantStr {
				t.Errorf("optText string = %q, want %q", got.String, tt.wantStr)
			}
		})
	}
}

func TestOptUUID(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		key       string
		wantValid bool
	}{
		{"valid uuid", map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000"}, "id", true},
		{"invalid uuid", map[string]any{"id": "not-a-uuid"}, "id", false},
		{"missing key", map[string]any{}, "id", false},
		{"nil value", map[string]any{"id": nil}, "id", false},
		{"empty string", map[string]any{"id": ""}, "id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optUUID(tt.args, tt.key)
			if got.Valid != tt.wantValid {
				t.Errorf("optUUID valid = %v, want %v", got.Valid, tt.wantValid)
			}
		})
	}
}
