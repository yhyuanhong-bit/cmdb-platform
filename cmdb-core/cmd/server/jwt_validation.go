package main

import (
	"fmt"
	"math"
)

// minJWTSecretBytes is the minimum acceptable JWT signing secret length.
// HS256 requires at least 32 bytes of keying material to resist brute-force;
// anything shorter is effectively guessable.
const minJWTSecretBytes = 32

// minJWTSecretEntropyBits is the minimum acceptable Shannon entropy (bits
// per byte). A truly random byte stream tops out at 8 bits; common weak
// secrets ("aaaaaa...", "password...", repeated phrases) fall well below
// 4.0 bits per byte, so 4.0 is a conservative lower bound that rejects the
// obvious foot-guns without being so strict that legitimate secrets fail.
const minJWTSecretEntropyBits = 4.0

// validateJWTSecret returns an error if the provided JWT signing secret is
// too short or has insufficient per-byte Shannon entropy. It is safe to
// call before the logger is initialized.
func validateJWTSecret(secret string) error {
	if len(secret) < minJWTSecretBytes {
		return fmt.Errorf("JWT_SECRET must be >= %d bytes (got %d)", minJWTSecretBytes, len(secret))
	}
	entropy := shannonEntropy([]byte(secret))
	if entropy < minJWTSecretEntropyBits {
		return fmt.Errorf(
			"JWT_SECRET has low entropy (%.2f bits/byte, need >= %.1f); do not reuse common values",
			entropy, minJWTSecretEntropyBits,
		)
	}
	return nil
}

// shannonEntropy computes the Shannon entropy (in bits per byte) of the
// input using the standard formula H = -Σ p_i * log2(p_i). Returns 0 for
// an empty input.
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	n := float64(len(data))
	var h float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}
