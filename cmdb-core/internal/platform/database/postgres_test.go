package database

import "testing"

func TestResolvePoolSize(t *testing.T) {
	const fallback int32 = 50

	tests := []struct {
		name string
		env  string
		want int32
	}{
		{name: "unset returns fallback", env: "", want: fallback},
		{name: "valid value", env: "100", want: 100},
		{name: "non-numeric falls back", env: "abc", want: fallback},
		{name: "zero is rejected", env: "0", want: fallback},
		{name: "negative is rejected", env: "-5", want: fallback},
		{name: "too large is rejected", env: "10001", want: fallback},
		{name: "upper bound allowed", env: "10000", want: 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("DB_TEST_POOL_SIZE", tt.env)
			} else {
				t.Setenv("DB_TEST_POOL_SIZE", "")
			}
			got := resolvePoolSize("DB_TEST_POOL_SIZE", fallback)
			if got != tt.want {
				t.Errorf("resolvePoolSize(env=%q) = %d, want %d", tt.env, got, tt.want)
			}
		})
	}
}
