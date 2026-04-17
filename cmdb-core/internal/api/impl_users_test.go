package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockIdentityService implements identityService for handler tests.
type mockIdentityService struct {
	listUsersFn       func(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.User, int64, error)
	getUserFn         func(ctx context.Context, id uuid.UUID) (*dbgen.User, error)
	createUserFn      func(ctx context.Context, params dbgen.CreateUserParams, pw string) (*dbgen.User, error)
	updateUserFn      func(ctx context.Context, params dbgen.UpdateUserParams) (*dbgen.User, error)
	listRolesFn       func(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Role, error)
	createRoleFn      func(ctx context.Context, params dbgen.CreateRoleParams) (*dbgen.Role, error)
	updateRoleFn      func(ctx context.Context, params dbgen.UpdateRoleParams) (*dbgen.Role, error)
	deleteRoleFn      func(ctx context.Context, tenantID, id uuid.UUID) error
	assignRoleFn      func(ctx context.Context, userID, roleID uuid.UUID) error
	removeRoleFn      func(ctx context.Context, userID, roleID uuid.UUID) error
	listUserRoleIDsFn func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	deactivateFn      func(ctx context.Context, tenantID, userID uuid.UUID) error
}

func (m *mockIdentityService) ListUsers(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.User, int64, error) {
	return m.listUsersFn(ctx, tenantID, limit, offset)
}
func (m *mockIdentityService) GetUser(ctx context.Context, id uuid.UUID) (*dbgen.User, error) {
	return m.getUserFn(ctx, id)
}
func (m *mockIdentityService) CreateUser(ctx context.Context, params dbgen.CreateUserParams, pw string) (*dbgen.User, error) {
	return m.createUserFn(ctx, params, pw)
}
func (m *mockIdentityService) UpdateUser(ctx context.Context, params dbgen.UpdateUserParams) (*dbgen.User, error) {
	return m.updateUserFn(ctx, params)
}
func (m *mockIdentityService) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Role, error) {
	return m.listRolesFn(ctx, tenantID)
}
func (m *mockIdentityService) CreateRole(ctx context.Context, params dbgen.CreateRoleParams) (*dbgen.Role, error) {
	return m.createRoleFn(ctx, params)
}
func (m *mockIdentityService) UpdateRole(ctx context.Context, params dbgen.UpdateRoleParams) (*dbgen.Role, error) {
	return m.updateRoleFn(ctx, params)
}
func (m *mockIdentityService) DeleteRole(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.deleteRoleFn(ctx, tenantID, id)
}
func (m *mockIdentityService) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return m.assignRoleFn(ctx, userID, roleID)
}
func (m *mockIdentityService) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return m.removeRoleFn(ctx, userID, roleID)
}
func (m *mockIdentityService) ListUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return m.listUserRoleIDsFn(ctx, userID)
}
func (m *mockIdentityService) Deactivate(ctx context.Context, tenantID, userID uuid.UUID) error {
	return m.deactivateFn(ctx, tenantID, userID)
}

// newUsersTestServer builds an APIServer with a mock identity service. auditSvc
// is left nil; recordAudit tolerates this via its nil-guard.
func newUsersTestServer(svc *mockIdentityService) *APIServer {
	return &APIServer{identitySvc: svc}
}

// runHandler wires a gin.Context with a request + params and invokes the handler.
// ctxParams contains gin URL parameter values (e.g., "id" → uuid).
func runHandler(t *testing.T, handler gin.HandlerFunc, method, target string, body any, ctxParams gin.Params, ctxSet map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = ctxParams
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
	req, _ := http.NewRequest(method, target, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	handler(c)
	// Flush any status that was set via c.Status() without a body write
	// (e.g., 204 No Content) so httptest.ResponseRecorder sees it.
	c.Writer.WriteHeaderNow()
	return rec
}

// ---------------------------------------------------------------------------
// AssignRoleToUser — security-critical
// ---------------------------------------------------------------------------

func TestAssignRoleToUser_Success(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()
	var capturedUser, capturedRole uuid.UUID
	svc := &mockIdentityService{
		assignRoleFn: func(_ context.Context, u, r uuid.UUID) error {
			capturedUser = u
			capturedRole = r
			return nil
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.AssignRoleToUser, http.MethodPost, "/users/"+userID.String()+"/roles",
		map[string]string{"role_id": roleID.String()},
		gin.Params{{Key: "id", Value: userID.String()}},
		map[string]string{"user_id": uuid.New().String(), "tenant_id": uuid.New().String()})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	if capturedUser != userID {
		t.Errorf("userID = %s, want %s", capturedUser, userID)
	}
	if capturedRole != roleID {
		t.Errorf("roleID = %s, want %s", capturedRole, roleID)
	}
}

func TestAssignRoleToUser_InvalidUserID(t *testing.T) {
	s := newUsersTestServer(&mockIdentityService{})
	rec := runHandler(t, s.AssignRoleToUser, http.MethodPost, "/users/not-a-uuid/roles",
		map[string]string{"role_id": uuid.New().String()},
		gin.Params{{Key: "id", Value: "not-a-uuid"}}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAssignRoleToUser_InvalidRoleID(t *testing.T) {
	userID := uuid.New()
	s := newUsersTestServer(&mockIdentityService{})
	rec := runHandler(t, s.AssignRoleToUser, http.MethodPost, "/users/"+userID.String()+"/roles",
		map[string]string{"role_id": "not-a-uuid"},
		gin.Params{{Key: "id", Value: userID.String()}}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAssignRoleToUser_MissingRoleID(t *testing.T) {
	userID := uuid.New()
	s := newUsersTestServer(&mockIdentityService{})
	rec := runHandler(t, s.AssignRoleToUser, http.MethodPost, "/users/"+userID.String()+"/roles",
		map[string]string{},
		gin.Params{{Key: "id", Value: userID.String()}}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAssignRoleToUser_ServiceError(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()
	svc := &mockIdentityService{
		assignRoleFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("db failure")
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.AssignRoleToUser, http.MethodPost, "/users/"+userID.String()+"/roles",
		map[string]string{"role_id": roleID.String()},
		gin.Params{{Key: "id", Value: userID.String()}},
		map[string]string{"user_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// RemoveRoleFromUser — security-critical
// ---------------------------------------------------------------------------

func TestRemoveRoleFromUser_Success(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()
	var capturedUser, capturedRole uuid.UUID
	svc := &mockIdentityService{
		removeRoleFn: func(_ context.Context, u, r uuid.UUID) error {
			capturedUser = u
			capturedRole = r
			return nil
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.RemoveRoleFromUser, http.MethodDelete,
		"/users/"+userID.String()+"/roles/"+roleID.String(), nil,
		gin.Params{{Key: "id", Value: userID.String()}, {Key: "roleId", Value: roleID.String()}},
		map[string]string{"user_id": uuid.New().String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204. body=%s", rec.Code, rec.Body.String())
	}
	if capturedUser != userID || capturedRole != roleID {
		t.Errorf("captured ids mismatch: user=%s role=%s", capturedUser, capturedRole)
	}
}

func TestRemoveRoleFromUser_InvalidIDs(t *testing.T) {
	s := newUsersTestServer(&mockIdentityService{})
	tests := []struct {
		name   string
		params gin.Params
	}{
		{"bad user id", gin.Params{{Key: "id", Value: "bad"}, {Key: "roleId", Value: uuid.New().String()}}},
		{"bad role id", gin.Params{{Key: "id", Value: uuid.New().String()}, {Key: "roleId", Value: "bad"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := runHandler(t, s.RemoveRoleFromUser, http.MethodDelete, "/", nil, tt.params, nil)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestRemoveRoleFromUser_ServiceError(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()
	svc := &mockIdentityService{
		removeRoleFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("db failure")
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.RemoveRoleFromUser, http.MethodDelete, "/", nil,
		gin.Params{{Key: "id", Value: userID.String()}, {Key: "roleId", Value: roleID.String()}},
		map[string]string{"user_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// ListUserRoles
// ---------------------------------------------------------------------------

func TestListUserRoles_Success(t *testing.T) {
	userID := uuid.New()
	roles := []uuid.UUID{uuid.New(), uuid.New()}
	svc := &mockIdentityService{
		listUserRoleIDsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if id != userID {
				t.Errorf("userID = %s, want %s", id, userID)
			}
			return roles, nil
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.ListUserRoles, http.MethodGet, "/users/"+userID.String()+"/roles",
		nil, gin.Params{{Key: "id", Value: userID.String()}}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var env struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Data) != 2 {
		t.Errorf("expected 2 role ids, got %d", len(env.Data))
	}
}

func TestListUserRoles_InvalidID(t *testing.T) {
	s := newUsersTestServer(&mockIdentityService{})
	rec := runHandler(t, s.ListUserRoles, http.MethodGet, "/users/bad/roles",
		nil, gin.Params{{Key: "id", Value: "bad"}}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListUserRoles_ServiceError(t *testing.T) {
	svc := &mockIdentityService{
		listUserRoleIDsFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
			return nil, errors.New("db failure")
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.ListUserRoles, http.MethodGet, "/",
		nil, gin.Params{{Key: "id", Value: uuid.New().String()}}, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// DeleteUser
// ---------------------------------------------------------------------------

func TestDeleteUser_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	var capturedTenant, capturedUser uuid.UUID
	svc := &mockIdentityService{
		deactivateFn: func(_ context.Context, tID, uID uuid.UUID) error {
			capturedTenant = tID
			capturedUser = uID
			return nil
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.DeleteUser, http.MethodDelete, "/users/"+userID.String(),
		nil, gin.Params{{Key: "id", Value: userID.String()}},
		map[string]string{"user_id": uuid.New().String(), "tenant_id": tenantID.String()})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204. body=%s", rec.Code, rec.Body.String())
	}
	if capturedTenant != tenantID || capturedUser != userID {
		t.Errorf("captured ids mismatch: tenant=%s user=%s", capturedTenant, capturedUser)
	}
}

func TestDeleteUser_InvalidID(t *testing.T) {
	s := newUsersTestServer(&mockIdentityService{})
	rec := runHandler(t, s.DeleteUser, http.MethodDelete, "/users/bad",
		nil, gin.Params{{Key: "id", Value: "bad"}}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDeleteUser_ServiceError(t *testing.T) {
	svc := &mockIdentityService{
		deactivateFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("db failure")
		},
	}
	s := newUsersTestServer(svc)
	rec := runHandler(t, s.DeleteUser, http.MethodDelete, "/",
		nil, gin.Params{{Key: "id", Value: uuid.New().String()}},
		map[string]string{"user_id": uuid.New().String(), "tenant_id": uuid.New().String()})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// DeleteRole — system roles are blocked at service layer; handler maps to 404.
// Takes IdPath directly rather than from gin params, so we call it with the
// typed wrapper.
// ---------------------------------------------------------------------------

func TestDeleteRole_Success(t *testing.T) {
	roleID := uuid.New()
	var capturedID uuid.UUID
	svc := &mockIdentityService{
		deleteRoleFn: func(_ context.Context, _, id uuid.UUID) error {
			capturedID = id
			return nil
		},
	}
	s := newUsersTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", uuid.New().String())
	c.Set("user_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodDelete, "/roles/"+roleID.String(), nil)
	c.Request = req
	s.DeleteRole(c, IdPath(roleID))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204. body=%s", rec.Code, rec.Body.String())
	}
	if capturedID != roleID {
		t.Errorf("roleID = %s, want %s", capturedID, roleID)
	}
}

func TestDeleteRole_SystemRoleBlocked(t *testing.T) {
	svc := &mockIdentityService{
		deleteRoleFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("cannot delete system role")
		},
	}
	s := newUsersTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodDelete, "/", nil)
	c.Request = req
	s.DeleteRole(c, IdPath(uuid.New()))
	c.Writer.WriteHeaderNow()

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// ListUsers — read path, no mutation
// ---------------------------------------------------------------------------

func TestListUsers_ServiceError(t *testing.T) {
	svc := &mockIdentityService{
		listUsersFn: func(_ context.Context, _ uuid.UUID, _, _ int32) ([]dbgen.User, int64, error) {
			return nil, 0, errors.New("db failure")
		},
	}
	s := newUsersTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	c.Request = req
	s.ListUsers(c, ListUsersParams{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListUsers_EmptyResult(t *testing.T) {
	svc := &mockIdentityService{
		listUsersFn: func(_ context.Context, _ uuid.UUID, _, _ int32) ([]dbgen.User, int64, error) {
			return []dbgen.User{}, 0, nil
		},
	}
	s := newUsersTestServer(svc)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("tenant_id", uuid.New().String())
	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	c.Request = req
	s.ListUsers(c, ListUsersParams{})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
}
