package main

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// TestValidateJWTSecret_RejectsShort verifies we reject secrets below the
// 32-byte minimum. Short secrets are trivially brute-forceable.
func TestValidateJWTSecret_RejectsShort(t *testing.T) {
	// 16 random bytes = 32 hex chars (well under the 32-byte minimum when
	// measured as raw bytes).
	short := make([]byte, 16)
	if _, err := rand.Read(short); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	err := validateJWTSecret(string(short))
	if err == nil {
		t.Fatalf("expected error for 16-byte secret, got nil")
	}
	if !strings.Contains(err.Error(), "32") {
		t.Errorf("error should mention minimum length 32: %v", err)
	}
}

// TestValidateJWTSecret_RejectsLowEntropy verifies we reject secrets that
// meet the length requirement but are trivially guessable (e.g. all 'a').
func TestValidateJWTSecret_RejectsLowEntropy(t *testing.T) {
	err := validateJWTSecret(strings.Repeat("a", 64))
	if err == nil {
		t.Fatalf("expected error for low-entropy secret, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "entropy") {
		t.Errorf("error should mention entropy: %v", err)
	}
}

// TestValidateJWTSecret_AcceptsHighEntropy verifies a cryptographically
// random 32-byte secret (base64-encoded for printability) passes validation.
// base64 uses a 64-symbol alphabet so the theoretical ceiling is 6 bits/byte,
// comfortably above the 4.0 bits/byte threshold even after sampling noise.
func TestValidateJWTSecret_AcceptsHighEntropy(t *testing.T) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	secret := base64.StdEncoding.EncodeToString(raw)
	if err := validateJWTSecret(secret); err != nil {
		t.Errorf("expected nil for high-entropy secret, got %v", err)
	}
}

// TestValidateJWTSecret_AcceptsRaw32Bytes verifies a 32-byte raw random
// secret (not hex-encoded) also passes.
func TestValidateJWTSecret_AcceptsRaw32Bytes(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if err := validateJWTSecret(string(raw)); err != nil {
		t.Errorf("expected nil for 32 random bytes, got %v", err)
	}
}

// TestShannonEntropy_KnownValues sanity-checks the entropy helper against
// hand-calculated values so regressions in the formula are caught.
func TestShannonEntropy_KnownValues(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
	}{
		// All-same byte => 0 bits of entropy.
		{"all same", strings.Repeat("a", 64), 0.0, 0.0001},
		// Two distinct bytes, equal frequency => 1 bit.
		{"two equal", strings.Repeat("ab", 32), 0.999, 1.001},
		// Four distinct bytes, equal frequency => 2 bits.
		{"four equal", strings.Repeat("abcd", 16), 1.999, 2.001},
	}
	for _, tt := range tests {
		got := shannonEntropy([]byte(tt.input))
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("%s: shannonEntropy = %v, want in [%v, %v]", tt.name, got, tt.wantMin, tt.wantMax)
		}
	}
}
