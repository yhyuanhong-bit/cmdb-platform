package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// extendedFakeQueries fleshes out the fakeIdentityQueries stub defined in
// service_test.go: it implements every method on identityQueries so we
// can drive the rest of Service through the same lightweight seam instead
// of panicking. Keeps service-level unit tests DB-free.
type extendedFakeQueries struct {
	users              map[uuid.UUID]dbgen.User
	roles              map[uuid.UUID]dbgen.Role
	userRoleIDs        map[uuid.UUID][]uuid.UUID
	listUsersResult    []dbgen.User
	listRolesResult    []dbgen.Role
	countUsersResult   int64
	listUsersErr       error
	countUsersErr      error
	getUserErr         error
	createUserErr      error
	updateUserErr      error
	deactivateUserErr  error
	listRolesErr       error
	getRoleErr         error
	createRoleErr      error
	updateRoleErr      error
	deleteRoleErr      error
	assignRoleErr      error
	removeRoleErr      error
	listUserRoleIDsErr error

	// captured inputs for assertions
	lastCreateUserParams    dbgen.CreateUserParams
	lastUpdateUserParams    dbgen.UpdateUserParams
	lastDeactivateParams    dbgen.DeactivateUserParams
	lastListUsersParams     dbgen.ListUsersParams
	lastCreateRoleParams    dbgen.CreateRoleParams
	lastUpdateRoleParams    dbgen.UpdateRoleParams
	lastDeleteRoleParams    dbgen.DeleteRoleParams
	lastAssignRoleParams    dbgen.AssignRoleParams
	lastRemoveRoleParams    dbgen.RemoveRoleParams
	lastListRolesTenantID   pgtype.UUID
	lastCountUsersTenantID  uuid.UUID
	lastListUserRoleIDsUser uuid.UUID
}

func newExtendedFake() *extendedFakeQueries {
	return &extendedFakeQueries{
		users:       make(map[uuid.UUID]dbgen.User),
		roles:       make(map[uuid.UUID]dbgen.Role),
		userRoleIDs: make(map[uuid.UUID][]uuid.UUID),
	}
}

func (f *extendedFakeQueries) ListUsers(_ context.Context, arg dbgen.ListUsersParams) ([]dbgen.User, error) {
	f.lastListUsersParams = arg
	if f.listUsersErr != nil {
		return nil, f.listUsersErr
	}
	return f.listUsersResult, nil
}

func (f *extendedFakeQueries) CountUsers(_ context.Context, tenantID uuid.UUID) (int64, error) {
	f.lastCountUsersTenantID = tenantID
	if f.countUsersErr != nil {
		return 0, f.countUsersErr
	}
	return f.countUsersResult, nil
}

func (f *extendedFakeQueries) GetUser(_ context.Context, id uuid.UUID) (dbgen.User, error) {
	if f.getUserErr != nil {
		return dbgen.User{}, f.getUserErr
	}
	u, ok := f.users[id]
	if !ok {
		return dbgen.User{}, errors.New("user not found")
	}
	return u, nil
}

// GetUserScoped is the tenant-scoped lookup the service now uses.
// Returns "user not found" when the row exists but belongs to a
// different tenant — same shape as the real SQL with a (id, tenant_id)
// WHERE pair.
func (f *extendedFakeQueries) GetUserScoped(_ context.Context, arg dbgen.GetUserScopedParams) (dbgen.User, error) {
	if f.getUserErr != nil {
		return dbgen.User{}, f.getUserErr
	}
	u, ok := f.users[arg.ID]
	if !ok || u.TenantID != arg.TenantID {
		return dbgen.User{}, errors.New("user not found in tenant")
	}
	return u, nil
}

func (f *extendedFakeQueries) CreateUser(_ context.Context, arg dbgen.CreateUserParams) (dbgen.User, error) {
	f.lastCreateUserParams = arg
	if f.createUserErr != nil {
		return dbgen.User{}, f.createUserErr
	}
	u := dbgen.User{
		ID:           uuid.New(),
		TenantID:     arg.TenantID,
		Username:     arg.Username,
		DisplayName:  arg.DisplayName,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		Status:       "active",
	}
	f.users[u.ID] = u
	return u, nil
}

func (f *extendedFakeQueries) UpdateUser(_ context.Context, arg dbgen.UpdateUserParams) (dbgen.User, error) {
	f.lastUpdateUserParams = arg
	if f.updateUserErr != nil {
		return dbgen.User{}, f.updateUserErr
	}
	u, ok := f.users[arg.ID]
	if !ok {
		return dbgen.User{}, errors.New("user not found")
	}
	// UpdateUserParams fields are pgtype.Text (nullable) so we only
	// copy the `.String` when `.Valid`. A naive unconditional copy
	// would blank the field for every NULL on every call.
	if arg.DisplayName.Valid {
		u.DisplayName = arg.DisplayName.String
	}
	if arg.Email.Valid {
		u.Email = arg.Email.String
	}
	f.users[arg.ID] = u
	return u, nil
}

func (f *extendedFakeQueries) DeactivateUser(_ context.Context, arg dbgen.DeactivateUserParams) error {
	f.lastDeactivateParams = arg
	return f.deactivateUserErr
}

func (f *extendedFakeQueries) ListRoles(_ context.Context, tenantID pgtype.UUID) ([]dbgen.Role, error) {
	f.lastListRolesTenantID = tenantID
	if f.listRolesErr != nil {
		return nil, f.listRolesErr
	}
	return f.listRolesResult, nil
}

func (f *extendedFakeQueries) GetRole(_ context.Context, id uuid.UUID) (dbgen.Role, error) {
	if f.getRoleErr != nil {
		return dbgen.Role{}, f.getRoleErr
	}
	r, ok := f.roles[id]
	if !ok {
		return dbgen.Role{}, errors.New("role not found")
	}
	return r, nil
}

func (f *extendedFakeQueries) CreateRole(_ context.Context, arg dbgen.CreateRoleParams) (dbgen.Role, error) {
	f.lastCreateRoleParams = arg
	if f.createRoleErr != nil {
		return dbgen.Role{}, f.createRoleErr
	}
	r := dbgen.Role{
		ID:          uuid.New(),
		TenantID:    arg.TenantID,
		Name:        arg.Name,
		Description: arg.Description,
		Permissions: arg.Permissions,
	}
	f.roles[r.ID] = r
	return r, nil
}

func (f *extendedFakeQueries) UpdateRole(_ context.Context, arg dbgen.UpdateRoleParams) (dbgen.Role, error) {
	f.lastUpdateRoleParams = arg
	if f.updateRoleErr != nil {
		return dbgen.Role{}, f.updateRoleErr
	}
	r, ok := f.roles[arg.ID]
	if !ok {
		return dbgen.Role{}, errors.New("role not found")
	}
	if arg.Name.Valid {
		r.Name = arg.Name.String
	}
	if arg.Description.Valid {
		r.Description = arg.Description
	}
	r.Permissions = arg.Permissions
	f.roles[arg.ID] = r
	return r, nil
}

func (f *extendedFakeQueries) DeleteRole(_ context.Context, arg dbgen.DeleteRoleParams) (int64, error) {
	f.lastDeleteRoleParams = arg
	if f.deleteRoleErr != nil {
		return 0, f.deleteRoleErr
	}
	if _, ok := f.roles[arg.ID]; !ok {
		return 0, nil
	}
	delete(f.roles, arg.ID)
	return 1, nil
}

func (f *extendedFakeQueries) AssignRole(_ context.Context, arg dbgen.AssignRoleParams) error {
	f.lastAssignRoleParams = arg
	if f.assignRoleErr != nil {
		return f.assignRoleErr
	}
	f.userRoleIDs[arg.UserID] = append(f.userRoleIDs[arg.UserID], arg.RoleID)
	return nil
}

func (f *extendedFakeQueries) RemoveRole(_ context.Context, arg dbgen.RemoveRoleParams) error {
	f.lastRemoveRoleParams = arg
	return f.removeRoleErr
}

func (f *extendedFakeQueries) ListUserRoleIDs(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	f.lastListUserRoleIDsUser = userID
	if f.listUserRoleIDsErr != nil {
		return nil, f.listUserRoleIDsErr
	}
	return f.userRoleIDs[userID], nil
}

// TestListUsers_HappyPath covers the pagination-relay contract: ListUsers
// forwards TenantID + Limit + Offset verbatim, follows up with a
// CountUsers(tenantID) call, and returns both results.
func TestListUsers_HappyPath(t *testing.T) {
	t.Parallel()

	q := newExtendedFake()
	tenantID := uuid.New()
	u1 := dbgen.User{ID: uuid.New(), TenantID: tenantID, Username: "alice"}
	u2 := dbgen.User{ID: uuid.New(), TenantID: tenantID, Username: "bob"}
	q.listUsersResult = []dbgen.User{u1, u2}
	q.countUsersResult = 42
	svc := &Service{queries: q}

	users, total, err := svc.ListUsers(context.Background(), tenantID, 50, 0)
	if err != nil {
		t.Fatalf("ListUsers err: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if total != 42 {
		t.Errorf("expected total=42, got %d", total)
	}
	if q.lastListUsersParams.TenantID != tenantID {
		t.Errorf("list params tenant mismatch: got %s want %s", q.lastListUsersParams.TenantID, tenantID)
	}
	if q.lastListUsersParams.Limit != 50 || q.lastListUsersParams.Offset != 0 {
		t.Errorf("pagination not forwarded: got limit=%d offset=%d",
			q.lastListUsersParams.Limit, q.lastListUsersParams.Offset)
	}
	if q.lastCountUsersTenantID != tenantID {
		t.Errorf("count tenant mismatch: got %s want %s", q.lastCountUsersTenantID, tenantID)
	}
}

// TestListUsers_ListErrorPropagates verifies the ListUsers failure path is
// wrapped and surfaced (not silently swallowed).
func TestListUsers_ListErrorPropagates(t *testing.T) {
	t.Parallel()
	q := newExtendedFake()
	q.listUsersErr = errors.New("pool timeout")
	svc := &Service{queries: q}

	_, _, err := svc.ListUsers(context.Background(), uuid.New(), 10, 0)
	if err == nil {
		t.Fatal("expected error when ListUsers query fails")
	}
}

// TestListUsers_CountErrorPropagates verifies the count failure path is
// wrapped. The list itself succeeded, but the contract demands both or
// nothing — a partial result would make the total misleading.
func TestListUsers_CountErrorPropagates(t *testing.T) {
	t.Parallel()
	q := newExtendedFake()
	q.listUsersResult = []dbgen.User{{ID: uuid.New()}}
	q.countUsersErr = errors.New("count down")
	svc := &Service{queries: q}

	_, _, err := svc.ListUsers(context.Background(), uuid.New(), 10, 0)
	if err == nil {
		t.Fatal("expected error when CountUsers fails")
	}
}

// TestGetUser covers both the success path and the not-found error. The
// pointer return means nil-checks on the caller side; ensure we never
// return a zero-value User as a "success".
func TestGetUser(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	want := dbgen.User{ID: userID, TenantID: tenantID, Username: "alice"}

	t.Run("found returns pointer", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.users[userID] = want
		svc := &Service{queries: q}

		got, err := svc.GetUser(context.Background(), tenantID, userID)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got == nil || got.ID != userID {
			t.Fatalf("expected pointer to user %s, got %+v", userID, got)
		}
	})

	t.Run("not found returns ErrUserNotFound", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		svc := &Service{queries: q}
		_, err := svc.GetUser(context.Background(), tenantID, uuid.New())
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got: %v", err)
		}
	})

	t.Run("cross-tenant returns ErrUserNotFound", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.users[userID] = want // user lives in tenantID
		svc := &Service{queries: q}

		// Caller passes a *different* tenant — must look like 404 to
		// avoid leaking cross-tenant existence.
		_, err := svc.GetUser(context.Background(), uuid.New(), userID)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound for cross-tenant call, got: %v", err)
		}
	})

	t.Run("query error mapped to ErrUserNotFound", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.getUserErr = errors.New("db blown up")
		svc := &Service{queries: q}
		_, err := svc.GetUser(context.Background(), tenantID, uuid.New())
		// All lookup failures (not-found, cross-tenant, db error) collapse
		// into ErrUserNotFound so the handler returns a uniform 404.
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got: %v", err)
		}
	})
}

// TestCreateUser_HashesPassword verifies the bcrypt contract: the service
// must NEVER pass a plaintext password into CreateUserParams. A regression
// that stores plaintext would be catastrophic, so lock the invariant here.
func TestCreateUser_HashesPassword(t *testing.T) {
	t.Parallel()

	q := newExtendedFake()
	svc := &Service{queries: q}
	plainPassword := "super-secret-password-123"
	params := dbgen.CreateUserParams{
		TenantID:    uuid.New(),
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
	}

	user, err := svc.CreateUser(context.Background(), params, plainPassword)
	if err != nil {
		t.Fatalf("CreateUser err: %v", err)
	}
	if user == nil {
		t.Fatal("expected user pointer")
	}
	// The CreateUserParams that reached the DB must carry a bcrypt hash,
	// never the plaintext.
	if q.lastCreateUserParams.PasswordHash == plainPassword {
		t.Fatal("PasswordHash is plaintext — bcrypt was bypassed")
	}
	if q.lastCreateUserParams.PasswordHash == "" {
		t.Fatal("PasswordHash is empty")
	}
	// Proving the stored hash actually validates against the plaintext
	// prevents a "just store any non-empty string" regression.
	if err := bcrypt.CompareHashAndPassword(
		[]byte(q.lastCreateUserParams.PasswordHash), []byte(plainPassword),
	); err != nil {
		t.Fatalf("bcrypt hash does not validate against plaintext: %v", err)
	}
}

// TestCreateUser_QueryErrorPropagates ensures the DB-layer failure is
// wrapped (not swallowed) and no user record is returned.
func TestCreateUser_QueryErrorPropagates(t *testing.T) {
	t.Parallel()
	q := newExtendedFake()
	q.createUserErr = errors.New("unique violation")
	svc := &Service{queries: q}

	got, err := svc.CreateUser(context.Background(), dbgen.CreateUserParams{
		TenantID: uuid.New(), Username: "x",
	}, "pw")
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if got != nil {
		t.Fatalf("expected nil user on error, got %+v", got)
	}
}

// TestUpdateUser exercises success + error branches and asserts params
// forwarding to the DB layer.
func TestUpdateUser(t *testing.T) {
	t.Parallel()

	t.Run("success forwards params", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		userID := uuid.New()
		q.users[userID] = dbgen.User{ID: userID, Username: "alice"}
		svc := &Service{queries: q}

		got, err := svc.UpdateUser(context.Background(), dbgen.UpdateUserParams{
			ID:          userID,
			DisplayName: pgtype.Text{String: "Alice Updated", Valid: true},
			Email:       pgtype.Text{String: "new@example.com", Valid: true},
		})
		if err != nil {
			t.Fatalf("UpdateUser err: %v", err)
		}
		if got.DisplayName != "Alice Updated" {
			t.Errorf("expected display name updated, got %s", got.DisplayName)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.updateUserErr = errors.New("conflict")
		svc := &Service{queries: q}
		_, err := svc.UpdateUser(context.Background(), dbgen.UpdateUserParams{ID: uuid.New()})
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestListRoles_ForwardsPgTenantID locks in the pgtype.UUID conversion
// (Valid=true, Bytes=tenantID). A future refactor that drops Valid would
// silently match NULL-tenant (system) roles only.
func TestListRoles_ForwardsPgTenantID(t *testing.T) {
	t.Parallel()

	q := newExtendedFake()
	tenantID := uuid.New()
	q.listRolesResult = []dbgen.Role{
		{ID: uuid.New(), Name: "admin"},
		{ID: uuid.New(), Name: "viewer"},
	}
	svc := &Service{queries: q}

	got, err := svc.ListRoles(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListRoles err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(got))
	}
	if !q.lastListRolesTenantID.Valid {
		t.Error("tenant UUID must be Valid=true; a Valid=false slot matches system roles only")
	}
	if uuid.UUID(q.lastListRolesTenantID.Bytes) != tenantID {
		t.Errorf("tenant bytes mismatch: got %s want %s",
			uuid.UUID(q.lastListRolesTenantID.Bytes), tenantID)
	}
}

// TestListRoles_ErrorPropagates: a transient DB failure must not be
// silently converted to "no roles" — that would drop every user's
// permissions.
func TestListRoles_ErrorPropagates(t *testing.T) {
	t.Parallel()
	q := newExtendedFake()
	q.listRolesErr = errors.New("db down")
	svc := &Service{queries: q}
	_, err := svc.ListRoles(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected wrapped error")
	}
}

// TestCreateRole covers happy path + error wrap.
func TestCreateRole(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		svc := &Service{queries: q}
		params := dbgen.CreateRoleParams{
			TenantID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Name:        "editor",
			Description: pgtype.Text{String: "edit perms", Valid: true},
			Permissions: []byte(`{"asset":["read","write"]}`),
		}
		r, err := svc.CreateRole(context.Background(), params)
		if err != nil {
			t.Fatalf("CreateRole err: %v", err)
		}
		if r.Name != "editor" {
			t.Errorf("name mismatch: got %s want editor", r.Name)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.createRoleErr = errors.New("unique violation")
		svc := &Service{queries: q}
		_, err := svc.CreateRole(context.Background(), dbgen.CreateRoleParams{})
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestUpdateRole covers happy path + error wrap.
func TestUpdateRole(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		roleID := uuid.New()
		q.roles[roleID] = dbgen.Role{ID: roleID, Name: "old"}
		svc := &Service{queries: q}
		_, err := svc.UpdateRole(context.Background(), dbgen.UpdateRoleParams{
			ID:   roleID,
			Name: pgtype.Text{String: "new", Valid: true},
		})
		if err != nil {
			t.Fatalf("UpdateRole err: %v", err)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.updateRoleErr = errors.New("not found")
		svc := &Service{queries: q}
		_, err := svc.UpdateRole(context.Background(), dbgen.UpdateRoleParams{ID: uuid.New()})
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestDeleteRole_ScopedToTenant asserts DeleteRole packs both ID and
// tenant_id into the params — without the tenant, this would be a
// cross-tenant deletion vector.
func TestDeleteRole_ScopedToTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	roleID := uuid.New()

	t.Run("success forwards tenant", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		// Pre-populate the role so the fake's DeleteRole returns rows=1.
		// W6.3: the service now treats rows=0 as ErrRoleNotFound, so the
		// fake must reflect a real DELETE-hit to test the success path.
		q.roles[roleID] = dbgen.Role{ID: roleID}
		svc := &Service{queries: q}
		err := svc.DeleteRole(context.Background(), tenantID, roleID)
		if err != nil {
			t.Fatalf("DeleteRole err: %v", err)
		}
		if q.lastDeleteRoleParams.ID != roleID {
			t.Errorf("role id mismatch: got %s want %s", q.lastDeleteRoleParams.ID, roleID)
		}
		if !q.lastDeleteRoleParams.TenantID.Valid {
			t.Fatal("tenant UUID must be Valid=true — a NULL slot matches system roles only")
		}
		if uuid.UUID(q.lastDeleteRoleParams.TenantID.Bytes) != tenantID {
			t.Errorf("tenant bytes mismatch: got %s want %s",
				uuid.UUID(q.lastDeleteRoleParams.TenantID.Bytes), tenantID)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.deleteRoleErr = errors.New("is_system = true")
		svc := &Service{queries: q}
		err := svc.DeleteRole(context.Background(), uuid.New(), uuid.New())
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestRemoveRole covers happy + error paths and forwards the (userID,
// roleID) tuple.
func TestRemoveRole(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		// Seed the user in our tenant so GetUserScoped finds it.
		q.users[userID] = dbgen.User{ID: userID, TenantID: tenantID}
		svc := &Service{queries: q}
		if err := svc.RemoveRole(context.Background(), tenantID, userID, roleID); err != nil {
			t.Fatalf("RemoveRole err: %v", err)
		}
		if q.lastRemoveRoleParams.UserID != userID || q.lastRemoveRoleParams.RoleID != roleID {
			t.Errorf("params mismatch: %+v", q.lastRemoveRoleParams)
		}
	})

	t.Run("cross-tenant returns ErrUserNotFound and skips RemoveRole", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		// User exists but in a *different* tenant.
		otherTenant := uuid.New()
		uid := uuid.New()
		q.users[uid] = dbgen.User{ID: uid, TenantID: otherTenant}
		svc := &Service{queries: q}

		err := svc.RemoveRole(context.Background(), tenantID, uid, roleID)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound for cross-tenant RemoveRole, got: %v", err)
		}
		// The downstream RemoveRole DB call must NOT have happened
		if q.lastRemoveRoleParams.UserID != uuid.Nil {
			t.Errorf("RemoveRole DB call must be skipped on cross-tenant; got %+v", q.lastRemoveRoleParams)
		}
	})

	t.Run("downstream error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		uid := uuid.New()
		q.users[uid] = dbgen.User{ID: uid, TenantID: tenantID}
		q.removeRoleErr = errors.New("fk violation")
		svc := &Service{queries: q}
		err := svc.RemoveRole(context.Background(), tenantID, uid, roleID)
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestListUserRoleIDs covers both the success path (returns configured
// IDs) and the error-propagation path.
func TestListUserRoleIDs(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	role1 := uuid.New()
	role2 := uuid.New()

	t.Run("returns configured ids", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.users[userID] = dbgen.User{ID: userID, TenantID: tenantID}
		q.userRoleIDs[userID] = []uuid.UUID{role1, role2}
		svc := &Service{queries: q}
		ids, err := svc.ListUserRoleIDs(context.Background(), tenantID, userID)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("want 2 ids got %d", len(ids))
		}
	})

	t.Run("cross-tenant returns ErrUserNotFound", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		uid := uuid.New()
		// User exists but in another tenant
		q.users[uid] = dbgen.User{ID: uid, TenantID: uuid.New()}
		q.userRoleIDs[uid] = []uuid.UUID{role1}
		svc := &Service{queries: q}

		_, err := svc.ListUserRoleIDs(context.Background(), tenantID, uid)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got: %v", err)
		}
	})

	t.Run("downstream error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		uid := uuid.New()
		q.users[uid] = dbgen.User{ID: uid, TenantID: tenantID}
		q.listUserRoleIDsErr = errors.New("pool closed")
		svc := &Service{queries: q}
		_, err := svc.ListUserRoleIDs(context.Background(), tenantID, uid)
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestDeactivate_ScopedToTenant asserts Deactivate forwards both id AND
// tenant_id — without the tenant, a cross-tenant actor could soft-delete
// any user by UUID guess.
func TestDeactivate_ScopedToTenant(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()

	t.Run("success forwards both ids", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		svc := &Service{queries: q}
		if err := svc.Deactivate(context.Background(), tenantID, userID); err != nil {
			t.Fatalf("Deactivate err: %v", err)
		}
		if q.lastDeactivateParams.ID != userID {
			t.Errorf("user id mismatch: got %s want %s", q.lastDeactivateParams.ID, userID)
		}
		if q.lastDeactivateParams.TenantID != tenantID {
			t.Errorf("tenant id mismatch: got %s want %s", q.lastDeactivateParams.TenantID, tenantID)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		t.Parallel()
		q := newExtendedFake()
		q.deactivateUserErr = errors.New("not found")
		svc := &Service{queries: q}
		err := svc.Deactivate(context.Background(), uuid.New(), uuid.New())
		if err == nil {
			t.Fatal("expected wrapped error")
		}
	})
}

// TestAssignRole_PropagatesQueryFailure covers the underlying AssignRole
// DB failure path — application-level checks passed, DB failed on INSERT.
func TestAssignRole_PropagatesQueryFailure(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()

	q := newExtendedFake()
	q.users[userID] = dbgen.User{ID: userID, TenantID: tenantID}
	q.roles[roleID] = dbgen.Role{ID: roleID, TenantID: pgtype.UUID{Bytes: tenantID, Valid: true}}
	q.assignRoleErr = errors.New("trigger rejected")

	svc := &Service{queries: q}
	err := svc.AssignRole(context.Background(), tenantID, userID, roleID)
	if err == nil {
		t.Fatal("expected wrapped error from AssignRole query failure")
	}
	if errors.Is(err, ErrCrossTenantRole) {
		t.Fatalf("query failure must NOT be confused with cross-tenant: %v", err)
	}
}

// TestNewService_WiresQueries is a light-weight smoke check: the
// constructor must not drop the queries handle. A regression here would
// make every method panic on first call.
func TestNewService_WiresQueries(t *testing.T) {
	t.Parallel()
	// We pass a nil *dbgen.Queries pointer — constructor must still
	// return a service (the panic would only fire when a method is
	// called, not at construction).
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}
