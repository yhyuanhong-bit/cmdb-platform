package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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

func TestValidateJWT_Valid(t *testing.T) {
	secret := "test-secret-key-for-jwt-validation"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"user_id":"b0000000-0000-0000-0000-000000000001","username":"admin","tenant_id":"a0000000-0000-0000-0000-000000000001","exp":` + fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()) + `}`))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := signingInput + "." + sig

	claims, err := validateJWT(token, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", claims.Username)
	}
	if claims.TenantID != "a0000000-0000-0000-0000-000000000001" {
		t.Errorf("expected tenant_id match, got %q", claims.TenantID)
	}
}

func TestValidateJWT_InvalidSignature(t *testing.T) {
	secret := "test-secret"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"user_id":"x","username":"admin","tenant_id":"y"}`))
	token := header + "." + payload + ".invalidsignature"

	_, err := validateJWT(token, secret)
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestValidateJWT_MalformedTokenTable(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dots", "nodots"},
		{"one dot", "one.dot"},
		{"four dots", "a.b.c.d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateJWT(tt.token, "secret")
			if err == nil {
				t.Errorf("expected error for malformed token %q", tt.token)
			}
		})
	}
}
