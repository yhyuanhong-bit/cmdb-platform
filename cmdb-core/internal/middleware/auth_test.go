package middleware

import (
	"testing"
	"time"
)

func TestGenerateAndValidateJWT(t *testing.T) {
	secret := "test-secret-that-is-at-least-32-chars"
	claims := JWTClaims{
		UserID:    "user-123",
		Username:  "admin",
		TenantID:  "tenant-456",
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := GenerateJWT(claims, secret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	parsed, err := validateJWT(token, secret)
	if err != nil {
		t.Fatalf("validateJWT failed: %v", err)
	}
	if parsed.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", parsed.UserID, claims.UserID)
	}
	if parsed.TenantID != claims.TenantID {
		t.Errorf("TenantID = %q, want %q", parsed.TenantID, claims.TenantID)
	}
	if parsed.Username != claims.Username {
		t.Errorf("Username = %q, want %q", parsed.Username, claims.Username)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	claims := JWTClaims{
		UserID:    "user-123",
		TenantID:  "tenant-456",
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}
	token, _ := GenerateJWT(claims, "correct-secret-at-least-32-chars!!")
	_, err := validateJWT(token, "wrong-secret-at-least-32-chars!!!")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateJWT_MalformedToken(t *testing.T) {
	_, err := validateJWT("not.a.valid.token", "secret")
	if err == nil {
		t.Fatal("expected error for 4-part token")
	}

	_, err = validateJWT("", "secret")
	if err == nil {
		t.Fatal("expected error for empty token")
	}

	_, err = validateJWT("only-one-part", "secret")
	if err == nil {
		t.Fatal("expected error for single-part token")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	// validateJWT itself does not check expiry -- the Auth middleware does.
	// We test that expired claims are correctly parsed and returned.
	claims := JWTClaims{
		UserID:    "user-123",
		TenantID:  "tenant-456",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	secret := "test-secret-that-is-at-least-32-chars"
	token, _ := GenerateJWT(claims, secret)
	parsed, err := validateJWT(token, secret)
	if err != nil {
		t.Fatalf("validateJWT should parse expired tokens: %v", err)
	}
	if parsed.ExpiresAt >= time.Now().Unix() {
		t.Error("expected ExpiresAt to be in the past")
	}
}
