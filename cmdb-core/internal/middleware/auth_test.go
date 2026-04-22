package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// ---------------------------------------------------------------------------
// Part A: JTI + IAT round-trip
// ---------------------------------------------------------------------------

const testSecret = "test-secret-that-is-at-least-32-chars"

func issueWithJTI(t *testing.T, id string, iat, exp time.Time) string {
	t.Helper()
	tok, err := GenerateJWT(JWTClaims{
		UserID:    "user-123",
		Username:  "admin",
		TenantID:  "tenant-456",
		ID:        id,
		IssuedAt:  iat.Unix(),
		ExpiresAt: exp.Unix(),
	}, testSecret)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	return tok
}

func TestJWT_RoundTripsJTI(t *testing.T) {
	jti := uuid.New().String()
	iat := time.Now()
	tok := issueWithJTI(t, jti, iat, iat.Add(15*time.Minute))

	parsed, err := validateJWT(tok, testSecret)
	if err != nil {
		t.Fatalf("validateJWT: %v", err)
	}
	if parsed.ID != jti {
		t.Errorf("jti = %q, want %q", parsed.ID, jti)
	}
	if _, err := uuid.Parse(parsed.ID); err != nil {
		t.Errorf("jti is not a valid uuid: %v", err)
	}
}

func TestJWT_RoundTripsIAT(t *testing.T) {
	iat := time.Now()
	tok := issueWithJTI(t, uuid.New().String(), iat, iat.Add(15*time.Minute))

	parsed, err := validateJWT(tok, testSecret)
	if err != nil {
		t.Fatalf("validateJWT: %v", err)
	}
	delta := parsed.IssuedAt - iat.Unix()
	if delta < -2 || delta > 2 {
		t.Errorf("iat drift = %ds, want <= 2s", delta)
	}
}

// ---------------------------------------------------------------------------
// Part C: middleware blacklist + password-change checks
// ---------------------------------------------------------------------------

type stubBlacklist struct {
	revoked map[string]bool
	err     error
}

func (s *stubBlacklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.revoked[jti], nil
}

type stubPwdChecker struct {
	changedAt time.Time
	err       error
}

func (s *stubPwdChecker) PasswordChangedAt(_ context.Context, _, _ string) (time.Time, error) {
	if s.err != nil {
		return time.Time{}, s.err
	}
	return s.changedAt, nil
}

func runAuth(t *testing.T, mw gin.HandlerFunc, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	c.Request = req
	mw(c)
	// If middleware aborted, handler chain ends here.
	if !c.IsAborted() {
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write([]byte(`{"ok":true}`))
	}
	return rec
}

func TestAuthMiddleware_RejectsRevokedJTI(t *testing.T) {
	jti := uuid.New().String()
	tok := issueWithJTI(t, jti, time.Now(), time.Now().Add(15*time.Minute))
	bl := &stubBlacklist{revoked: map[string]bool{jti: true}}

	mw := Auth(testSecret, WithBlacklist(bl))
	rec := runAuth(t, mw, tok)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Error.Code != "TOKEN_REVOKED" {
		t.Errorf("error.code = %q, want TOKEN_REVOKED", body.Error.Code)
	}
}

func TestAuthMiddleware_AllowsUnrevokedJTI(t *testing.T) {
	jti := uuid.New().String()
	tok := issueWithJTI(t, jti, time.Now(), time.Now().Add(15*time.Minute))
	bl := &stubBlacklist{revoked: map[string]bool{}}

	mw := Auth(testSecret, WithBlacklist(bl))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_BlacklistErrorFailsClosedByDefault(t *testing.T) {
	jti := uuid.New().String()
	tok := issueWithJTI(t, jti, time.Now(), time.Now().Add(15*time.Minute))
	bl := &stubBlacklist{err: errors.New("redis down")}

	// Default policy is "closed": a Redis outage must not let tokens slip
	// through. Production should alert on the 503, not on the 200 response
	// the old fail-open behaviour produced.
	mw := Auth(testSecret, WithBlacklist(bl))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (fail-closed) on blacklist error, got %d", rec.Code)
	}
}

func TestAuthMiddleware_BlacklistErrorFailsOpenWhenConfigured(t *testing.T) {
	t.Setenv("AUTH_FAIL_POLICY", "open")
	jti := uuid.New().String()
	tok := issueWithJTI(t, jti, time.Now(), time.Now().Add(15*time.Minute))
	bl := &stubBlacklist{err: errors.New("redis down")}

	mw := Auth(testSecret, WithBlacklist(bl))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open on blacklist error with AUTH_FAIL_POLICY=open, got %d", rec.Code)
	}
}

func TestAuthMiddleware_RejectsTokenIssuedBeforePasswordChange(t *testing.T) {
	iat := time.Now().Add(-10 * time.Minute)
	tok := issueWithJTI(t, uuid.New().String(), iat, time.Now().Add(5*time.Minute))
	pwd := &stubPwdChecker{changedAt: time.Now().Add(-5 * time.Minute)}

	mw := Auth(testSecret, WithPasswordChangeChecker(pwd))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "TOKEN_OUTDATED" {
		t.Errorf("error.code = %q, want TOKEN_OUTDATED", body.Error.Code)
	}
}

func TestAuthMiddleware_AllowsTokenIssuedAfterPasswordChange(t *testing.T) {
	pwdChanged := time.Now().Add(-10 * time.Minute)
	tok := issueWithJTI(t, uuid.New().String(), time.Now(), time.Now().Add(5*time.Minute))
	pwd := &stubPwdChecker{changedAt: pwdChanged}

	mw := Auth(testSecret, WithPasswordChangeChecker(pwd))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_PasswordCheckerErrorFailsClosedByDefault(t *testing.T) {
	tok := issueWithJTI(t, uuid.New().String(), time.Now(), time.Now().Add(5*time.Minute))
	pwd := &stubPwdChecker{err: errors.New("db down")}

	mw := Auth(testSecret, WithPasswordChangeChecker(pwd))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (fail-closed) on pwd-checker error, got %d", rec.Code)
	}
}

func TestAuthMiddleware_PasswordCheckerErrorFailsOpenWhenConfigured(t *testing.T) {
	t.Setenv("AUTH_FAIL_POLICY", "open")
	tok := issueWithJTI(t, uuid.New().String(), time.Now(), time.Now().Add(5*time.Minute))
	pwd := &stubPwdChecker{err: errors.New("db down")}

	mw := Auth(testSecret, WithPasswordChangeChecker(pwd))
	rec := runAuth(t, mw, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open on pwd-checker error with AUTH_FAIL_POLICY=open, got %d", rec.Code)
	}
}

func TestAuthMiddleware_SetsJTIAndExpOnContext(t *testing.T) {
	jti := uuid.New().String()
	exp := time.Now().Add(15 * time.Minute)
	tok := issueWithJTI(t, jti, time.Now(), exp)

	mw := Auth(testSecret)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	c.Request = req
	mw(c)

	if c.IsAborted() {
		t.Fatalf("middleware aborted unexpectedly. body=%s", rec.Body.String())
	}
	if got := c.GetString("jti"); got != jti {
		t.Errorf("context jti = %q, want %q", got, jti)
	}
	if got, _ := c.Get("token_exp"); got.(int64) != exp.Unix() {
		t.Errorf("context token_exp = %v, want %d", got, exp.Unix())
	}
}
