package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockAuthService implements authService for handler tests.
type mockAuthService struct {
	loginFn          func(ctx context.Context, req identity.LoginRequest) (*identity.TokenResponse, error)
	refreshFn        func(ctx context.Context, refreshToken string) (*identity.TokenResponse, error)
	getCurrentUserFn func(ctx context.Context, userID string) (*identity.CurrentUser, error)
	changePasswordFn func(ctx context.Context, userID uuid.UUID, curr, next string) error
	logoutFn         func(ctx context.Context, userID uuid.UUID, jti string, exp time.Time) error
}

func (m *mockAuthService) Login(ctx context.Context, req identity.LoginRequest) (*identity.TokenResponse, error) {
	if m.loginFn == nil {
		return nil, errors.New("mock: Login not implemented")
	}
	return m.loginFn(ctx, req)
}

func (m *mockAuthService) Refresh(ctx context.Context, refreshToken string) (*identity.TokenResponse, error) {
	if m.refreshFn == nil {
		return nil, errors.New("mock: Refresh not implemented")
	}
	return m.refreshFn(ctx, refreshToken)
}

func (m *mockAuthService) GetCurrentUser(ctx context.Context, userID string) (*identity.CurrentUser, error) {
	if m.getCurrentUserFn == nil {
		return nil, errors.New("mock: GetCurrentUser not implemented")
	}
	return m.getCurrentUserFn(ctx, userID)
}

func (m *mockAuthService) ChangePassword(ctx context.Context, userID uuid.UUID, curr, next string) error {
	if m.changePasswordFn == nil {
		return errors.New("mock: ChangePassword not implemented")
	}
	return m.changePasswordFn(ctx, userID, curr, next)
}

func (m *mockAuthService) Logout(ctx context.Context, userID uuid.UUID, jti string, exp time.Time) error {
	if m.logoutFn == nil {
		return nil
	}
	return m.logoutFn(ctx, userID, jti, exp)
}

func newTestServer(svc *mockAuthService) *APIServer {
	return &APIServer{authSvc: svc}
}

func performJSON(t *testing.T, handler gin.HandlerFunc, method, path string, body any, ctxSet map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	for k, v := range ctxSet {
		c.Set(k, v)
	}
	var reqBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reqBody = b
	}
	req, _ := http.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	handler(c)
	return rec
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(_ context.Context, req identity.LoginRequest) (*identity.TokenResponse, error) {
			if req.Username != "admin" || req.Password != "hunter2" {
				t.Errorf("unexpected credentials: %+v", req)
			}
			return &identity.TokenResponse{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    3600,
			}, nil
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.Login, http.MethodPost, "/auth/login",
		LoginRequest{Username: "admin", Password: "hunter2"}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data TokenPair `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Data.AccessToken != "access-token" || env.Data.ExpiresIn != 3600 {
		t.Errorf("unexpected TokenPair: %+v", env.Data)
	}
}

func TestLogin_InvalidBody(t *testing.T) {
	s := newTestServer(&mockAuthService{})
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	s.Login(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLogin_ServiceError(t *testing.T) {
	svc := &mockAuthService{
		loginFn: func(_ context.Context, _ identity.LoginRequest) (*identity.TokenResponse, error) {
			return nil, errors.New("invalid credentials")
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.Login, http.MethodPost, "/auth/login",
		LoginRequest{Username: "admin", Password: "wrong"}, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401. body=%s", rec.Code, rec.Body.String())
	}
}

// TestLogin_PropagatesTenantSlug guards the HTTP→domain hand-off for
// Phase 1.3. The handler must forward tenant_slug from the request body
// into identity.LoginRequest, otherwise the disambiguation feature is
// invisible to clients.
func TestLogin_PropagatesTenantSlug(t *testing.T) {
	var captured identity.LoginRequest
	svc := &mockAuthService{
		loginFn: func(_ context.Context, req identity.LoginRequest) (*identity.TokenResponse, error) {
			captured = req
			return &identity.TokenResponse{AccessToken: "t", RefreshToken: "r", ExpiresIn: 1}, nil
		},
	}
	s := newTestServer(svc)
	slug := "acme-corp"
	rec := performJSON(t, s.Login, http.MethodPost, "/auth/login",
		LoginRequest{TenantSlug: &slug, Username: "admin", Password: "pw"}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	if captured.TenantSlug != "acme-corp" {
		t.Errorf("TenantSlug = %q, want acme-corp", captured.TenantSlug)
	}
}

func TestLogin_PropagatesClientIPAndUserAgent(t *testing.T) {
	var captured identity.LoginRequest
	svc := &mockAuthService{
		loginFn: func(_ context.Context, req identity.LoginRequest) (*identity.TokenResponse, error) {
			captured = req
			return &identity.TokenResponse{AccessToken: "t", RefreshToken: "r", ExpiresIn: 1}, nil
		},
	}
	s := newTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "pw"})
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "unit-test-agent/1.0")
	req.RemoteAddr = "203.0.113.42:54321"
	c.Request = req
	s.Login(c)

	if captured.UserAgent != "unit-test-agent/1.0" {
		t.Errorf("UserAgent not propagated: %q", captured.UserAgent)
	}
	if captured.ClientIP != "203.0.113.42" {
		t.Errorf("ClientIP = %q, want 203.0.113.42", captured.ClientIP)
	}
}

// ---------------------------------------------------------------------------
// RefreshToken
// ---------------------------------------------------------------------------

func TestRefreshToken_Success(t *testing.T) {
	svc := &mockAuthService{
		refreshFn: func(_ context.Context, token string) (*identity.TokenResponse, error) {
			if token != "old-refresh" {
				t.Errorf("refreshToken = %q, want old-refresh", token)
			}
			return &identity.TokenResponse{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
				ExpiresIn:    1800,
			}, nil
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.RefreshToken, http.MethodPost, "/auth/refresh",
		RefreshRequest{RefreshToken: "old-refresh"}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data TokenPair `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want new-access", env.Data.AccessToken)
	}
}

func TestRefreshToken_InvalidBody(t *testing.T) {
	s := newTestServer(&mockAuthService{})
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	s.RefreshToken(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRefreshToken_ServiceError(t *testing.T) {
	svc := &mockAuthService{
		refreshFn: func(_ context.Context, _ string) (*identity.TokenResponse, error) {
			return nil, errors.New("refresh token expired")
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.RefreshToken, http.MethodPost, "/auth/refresh",
		RefreshRequest{RefreshToken: "expired"}, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GetCurrentUser
// ---------------------------------------------------------------------------

func TestGetCurrentUser_Success(t *testing.T) {
	userID := uuid.New()
	svc := &mockAuthService{
		getCurrentUserFn: func(_ context.Context, id string) (*identity.CurrentUser, error) {
			if id != userID.String() {
				t.Errorf("userID = %q, want %q", id, userID.String())
			}
			return &identity.CurrentUser{
				ID:          userID,
				Username:    "admin",
				DisplayName: "Admin User",
				Email:       "admin@example.com",
				Permissions: map[string][]string{"assets": {"read", "write"}},
			}, nil
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.GetCurrentUser, http.MethodGet, "/auth/me", nil,
		map[string]string{"user_id": userID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data CurrentUser `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Data.Username != "admin" {
		t.Errorf("Username = %q, want admin", env.Data.Username)
	}
	if len(env.Data.Permissions["assets"]) != 2 {
		t.Errorf("expected 2 asset permissions, got %d", len(env.Data.Permissions["assets"]))
	}
}

func TestGetCurrentUser_ServiceError(t *testing.T) {
	svc := &mockAuthService{
		getCurrentUserFn: func(_ context.Context, _ string) (*identity.CurrentUser, error) {
			return nil, errors.New("user not found")
		},
	}
	s := newTestServer(svc)
	rec := performJSON(t, s.GetCurrentUser, http.MethodGet, "/auth/me", nil,
		map[string]string{"user_id": uuid.New().String()})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// ChangePassword (impl_sessions.go)
// ---------------------------------------------------------------------------

func TestChangePassword_Success(t *testing.T) {
	userID := uuid.New()
	var capturedID uuid.UUID
	var capturedCurr, capturedNext string
	svc := &mockAuthService{
		changePasswordFn: func(_ context.Context, id uuid.UUID, curr, next string) error {
			capturedID = id
			capturedCurr = curr
			capturedNext = next
			return nil
		},
	}
	s := newTestServer(svc)
	body := map[string]string{
		"current_password": "old-pw",
		"new_password":     "new-pw",
	}
	rec := performJSON(t, s.ChangePassword, http.MethodPost, "/auth/change-password", body,
		map[string]string{"user_id": userID.String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	if capturedID != userID {
		t.Errorf("userID = %s, want %s", capturedID, userID)
	}
	if capturedCurr != "old-pw" || capturedNext != "new-pw" {
		t.Errorf("passwords not propagated: curr=%q next=%q", capturedCurr, capturedNext)
	}
}

func TestChangePassword_MissingFields(t *testing.T) {
	s := newTestServer(&mockAuthService{})
	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing current", map[string]string{"new_password": "new"}},
		{"missing new", map[string]string{"current_password": "old"}},
		{"both empty", map[string]string{"current_password": "", "new_password": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performJSON(t, s.ChangePassword, http.MethodPost, "/auth/change-password",
				tt.body, map[string]string{"user_id": uuid.New().String()})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestChangePassword_ServiceError(t *testing.T) {
	svc := &mockAuthService{
		changePasswordFn: func(_ context.Context, _ uuid.UUID, _, _ string) error {
			return errors.New("current password incorrect")
		},
	}
	s := newTestServer(svc)
	body := map[string]string{
		"current_password": "wrong",
		"new_password":     "new",
	}
	rec := performJSON(t, s.ChangePassword, http.MethodPost, "/auth/change-password", body,
		map[string]string{"user_id": uuid.New().String()})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401. body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

func TestLogout_Success(t *testing.T) {
	userID := uuid.New()
	jti := uuid.New().String()
	exp := time.Now().Add(5 * time.Minute)

	var capturedUID uuid.UUID
	var capturedJTI string
	var capturedExp time.Time
	svc := &mockAuthService{
		logoutFn: func(_ context.Context, uid uuid.UUID, j string, e time.Time) error {
			capturedUID = uid
			capturedJTI = j
			capturedExp = e
			return nil
		},
	}
	s := newTestServer(svc)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", userID.String())
	c.Set("jti", jti)
	c.Set("token_exp", exp.Unix())
	req, _ := http.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Request = req
	s.Logout(c)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204. body=%s", rec.Code, rec.Body.String())
	}
	if capturedUID != userID {
		t.Errorf("userID = %s, want %s", capturedUID, userID)
	}
	if capturedJTI != jti {
		t.Errorf("jti = %q, want %q", capturedJTI, jti)
	}
	if capturedExp.Unix() != exp.Unix() {
		t.Errorf("exp = %d, want %d", capturedExp.Unix(), exp.Unix())
	}
}

func TestLogout_ServiceError(t *testing.T) {
	svc := &mockAuthService{
		logoutFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
			return errors.New("redis down")
		},
	}
	s := newTestServer(svc)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", uuid.New().String())
	c.Set("jti", "some-jti")
	c.Set("token_exp", time.Now().Add(5*time.Minute).Unix())
	req, _ := http.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Request = req
	s.Logout(c)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestLogout_FallsBackWhenTokenExpMissing(t *testing.T) {
	// Defensive: when the middleware chain didn't populate token_exp (e.g.
	// legacy tokens), the handler should still succeed with a safe default.
	var capturedExp time.Time
	svc := &mockAuthService{
		logoutFn: func(_ context.Context, _ uuid.UUID, _ string, e time.Time) error {
			capturedExp = e
			return nil
		},
	}
	s := newTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("user_id", uuid.New().String())
	c.Set("jti", "jti")
	req, _ := http.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Request = req
	s.Logout(c)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if capturedExp.IsZero() {
		t.Error("handler should supply a non-zero fallback exp")
	}
}
