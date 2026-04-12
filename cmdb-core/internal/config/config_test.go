package config

import (
	"os"
	"testing"
)

func TestEnvOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		def      string
		expected string
	}{
		{"uses default when unset", "TEST_CONFIG_UNSET_VAR", "", "default_val", "default_val"},
		{"uses env when set", "TEST_CONFIG_SET_VAR", "custom", "default_val", "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}
			got := envOrDefault(tt.envKey, tt.def)
			if got != tt.expected {
				t.Errorf("envOrDefault(%q) = %q, want %q", tt.envKey, got, tt.expected)
			}
		})
	}
}

func TestEnvOrDefaultInt(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		def      int
		expected int
	}{
		{"uses default when unset", "TEST_INT_UNSET", "", 500, 500},
		{"uses env when set", "TEST_INT_SET", "1000", 500, 1000},
		{"uses default on invalid", "TEST_INT_BAD", "notanumber", 500, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}
			got := envOrDefaultInt(tt.envKey, tt.def)
			if got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestEnvOrDefaultFloat(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		def      float64
		expected float64
	}{
		{"uses default when unset", "TEST_FLOAT_UNSET", "", 0.0005, 0.0005},
		{"uses env when set", "TEST_FLOAT_SET", "0.001", 0.0005, 0.001},
		{"uses default on invalid", "TEST_FLOAT_INVALID", "notanumber", 0.0005, 0.0005},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}
			got := envOrDefaultFloat(tt.envKey, tt.def)
			if got != tt.expected {
				t.Errorf("envOrDefaultFloat(%q) = %f, want %f", tt.envKey, got, tt.expected)
			}
		})
	}
}
