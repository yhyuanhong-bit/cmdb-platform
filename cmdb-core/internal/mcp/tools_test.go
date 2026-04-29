package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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

// TestParseTenantIDArg verifies the pure parsing logic that backs
// MCPServer.resolveTenantID. The DB-backed default-tenant fallback is not
// exercised here (it requires a live Queries instance) — only the arg
// parsing branches are unit-tested.
func TestParseTenantIDArg(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	parsedValid := uuid.MustParse(validUUID)

	tests := []struct {
		name        string
		args        map[string]any
		wantID      uuid.UUID
		wantPresent bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid uuid string is returned as present",
			args:        map[string]any{"tenant_id": validUUID},
			wantID:      parsedValid,
			wantPresent: true,
			wantErr:     false,
		},
		{
			name:        "invalid uuid string returns error",
			args:        map[string]any{"tenant_id": "not-a-uuid"},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     true,
			errContains: "invalid tenant_id UUID",
		},
		{
			name:        "missing tenant_id key falls back (present=false, no error)",
			args:        map[string]any{},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name:        "nil tenant_id falls back",
			args:        map[string]any{"tenant_id": nil},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name:        "empty string tenant_id falls back",
			args:        map[string]any{"tenant_id": ""},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name:        "non-string tenant_id (int) returns error",
			args:        map[string]any{"tenant_id": 42},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     true,
			errContains: "tenant_id must be a UUID string",
		},
		{
			name:        "non-string tenant_id (bool) returns error",
			args:        map[string]any{"tenant_id": true},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     true,
			errContains: "tenant_id must be a UUID string",
		},
		{
			name:        "non-string tenant_id (map) returns error",
			args:        map[string]any{"tenant_id": map[string]any{"k": "v"}},
			wantID:      uuid.Nil,
			wantPresent: false,
			wantErr:     true,
			errContains: "tenant_id must be a UUID string",
		},
		{
			name:        "uppercase uuid is accepted (uuid.Parse is case-insensitive)",
			args:        map[string]any{"tenant_id": strings.ToUpper(validUUID)},
			wantID:      parsedValid,
			wantPresent: true,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotPresent, err := parseTenantIDArg(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPresent != tt.wantPresent {
				t.Errorf("present = %v, want %v", gotPresent, tt.wantPresent)
			}
			if gotID != tt.wantID {
				t.Errorf("id = %v, want %v", gotID, tt.wantID)
			}
		})
	}
}

// TestResolveTenantID_ExplicitOverride verifies that resolveTenantID returns
// the parsed UUID without consulting the database when a valid tenant_id arg
// is supplied. We use a nil-queries MCPServer to prove the DB path is not
// reached in this branch — if it were, the call would panic.
func TestResolveTenantID_ExplicitOverride(t *testing.T) {
	srv := &MCPServer{queries: nil}
	want := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	got, err := srv.resolveTenantID(nil, map[string]any{"tenant_id": want.String()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestResolveTenantID_InvalidArgShortCircuits verifies that an invalid
// tenant_id arg surfaces an error before the DB is consulted.
func TestResolveTenantID_InvalidArgShortCircuits(t *testing.T) {
	srv := &MCPServer{queries: nil}
	_, err := srv.resolveTenantID(nil, map[string]any{"tenant_id": "not-a-uuid"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid tenant_id UUID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestResolveTenantID_NonStringShortCircuits verifies the wrong-type error
// path also avoids the DB.
func TestResolveTenantID_NonStringShortCircuits(t *testing.T) {
	srv := &MCPServer{queries: nil}
	_, err := srv.resolveTenantID(nil, map[string]any{"tenant_id": 123})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tenant_id must be a UUID string") {
		t.Errorf("unexpected error message: %v", err)
	}
}
