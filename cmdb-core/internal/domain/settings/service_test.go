package settings

import (
	"errors"
	"testing"
)

// These tests cover the pure business logic on AssetLifespanConfig.
// Database-touching paths in service.go are exercised by the integration
// tests under internal/api/impl_settings_test.go which spin up the
// in-process Postgres harness.

func TestDefaultAssetLifespan_MatchesPreviousHardcodedValues(t *testing.T) {
	d := DefaultAssetLifespan()
	if d.Server != 5 {
		t.Errorf("server default = %d, want 5", d.Server)
	}
	if d.Network != 7 {
		t.Errorf("network default = %d, want 7", d.Network)
	}
	if d.Storage != 5 {
		t.Errorf("storage default = %d, want 5", d.Storage)
	}
	if d.Power != 10 {
		t.Errorf("power default = %d, want 10", d.Power)
	}
}

func TestGetForType_FallbackToDefaults(t *testing.T) {
	empty := AssetLifespanConfig{}
	tests := []struct {
		assetType string
		want      int64
	}{
		{"server", 5},
		{"rack_server", 5},
		{"NETWORK", 7},
		{"network_switch", 7},
		{"storage", 5},
		{"power", 10},
		{"unknown", 5}, // DefaultLifespanFallbackYears
		{"", 5},
	}
	for _, tt := range tests {
		t.Run(tt.assetType, func(t *testing.T) {
			got := empty.GetForType(tt.assetType)
			if got != tt.want {
				t.Errorf("GetForType(%q) = %d, want %d", tt.assetType, got, tt.want)
			}
		})
	}
}

func TestGetForType_CustomOverridesDefault(t *testing.T) {
	cfg := AssetLifespanConfig{Server: 8, Network: 12}
	if got := cfg.GetForType("server"); got != 8 {
		t.Errorf("custom server = %d, want 8", got)
	}
	if got := cfg.GetForType("network"); got != 12 {
		t.Errorf("custom network = %d, want 12", got)
	}
	// Unset fields fall back to defaults, not zero.
	if got := cfg.GetForType("storage"); got != 5 {
		t.Errorf("storage fallback = %d, want 5", got)
	}
	if got := cfg.GetForType("power"); got != 10 {
		t.Errorf("power fallback = %d, want 10", got)
	}
}

func TestMergedWithDefaults_FillsZeroFields(t *testing.T) {
	merged := AssetLifespanConfig{Server: 9}.MergedWithDefaults()
	if merged.Server != 9 {
		t.Errorf("server preserved = %d, want 9", merged.Server)
	}
	if merged.Network != 7 || merged.Storage != 5 || merged.Power != 10 {
		t.Errorf("defaults backfilled wrong: %+v", merged)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       AssetLifespanConfig
		wantField string
	}{
		{"all_zero_ok", AssetLifespanConfig{}, ""},
		{"all_in_range", AssetLifespanConfig{Server: 5, Network: 7, Storage: 5, Power: 10}, ""},
		{"server_too_low", AssetLifespanConfig{Server: 0 /* zero is allowed */}, ""},
		{"server_below_min", AssetLifespanConfig{Server: -1}, "server"},
		{"network_above_max", AssetLifespanConfig{Network: 31}, "network"},
		{"storage_at_max", AssetLifespanConfig{Storage: 30}, ""},
		{"power_at_min", AssetLifespanConfig{Power: 1}, ""},
		{"power_above_max", AssetLifespanConfig{Power: 100}, "power"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, _ := tt.cfg.Validate()
			if field != tt.wantField {
				t.Errorf("Validate() field = %q, want %q", field, tt.wantField)
			}
		})
	}
}

func TestNewService_PanicsOnNilPool(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewService(nil) did not panic")
		}
	}()
	_ = NewService(nil)
}

func TestErrInvalidLifespan_IsExported(t *testing.T) {
	// Sanity: the handler relies on errors.Is(err, ErrInvalidLifespan)
	// to map to HTTP 400. Make sure it stays referenceable.
	wrapped := errors.New("wrapped")
	if errors.Is(wrapped, ErrInvalidLifespan) {
		t.Fatal("unrelated error should not match ErrInvalidLifespan")
	}
}

func TestDefaults_ReturnsCanonicalSettings(t *testing.T) {
	s := Defaults()
	if s.AssetLifespan != DefaultAssetLifespan() {
		t.Errorf("Defaults().AssetLifespan = %+v, want %+v", s.AssetLifespan, DefaultAssetLifespan())
	}
}
