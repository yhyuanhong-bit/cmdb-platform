package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// fakeIdentityQueries is a minimal stand-in for *dbgen.Queries. Only the
// methods exercised by AssignRole have meaningful implementations; everything
// else panics to make accidental reliance obvious.
type fakeIdentityQueries struct {
	users         map[uuid.UUID]dbgen.User
	roles         map[uuid.UUID]dbgen.Role
	assignCalls   []dbgen.AssignRoleParams
	getUserErr    error
	getRoleErr    error
	assignRoleErr error
}

func newFakeQueries() *fakeIdentityQueries {
	return &fakeIdentityQueries{
		users: make(map[uuid.UUID]dbgen.User),
		roles: make(map[uuid.UUID]dbgen.Role),
	}
}

func (f *fakeIdentityQueries) GetUser(_ context.Context, id uuid.UUID) (dbgen.User, error) {
	if f.getUserErr != nil {
		return dbgen.User{}, f.getUserErr
	}
	u, ok := f.users[id]
	if !ok {
		return dbgen.User{}, errors.New("user not found")
	}
	return u, nil
}

func (f *fakeIdentityQueries) GetRole(_ context.Context, id uuid.UUID) (dbgen.Role, error) {
	if f.getRoleErr != nil {
		return dbgen.Role{}, f.getRoleErr
	}
	r, ok := f.roles[id]
	if !ok {
		return dbgen.Role{}, errors.New("role not found")
	}
	return r, nil
}

func (f *fakeIdentityQueries) AssignRole(_ context.Context, arg dbgen.AssignRoleParams) error {
	if f.assignRoleErr != nil {
		return f.assignRoleErr
	}
	f.assignCalls = append(f.assignCalls, arg)
	return nil
}

// Unused methods — panic to catch accidental use in tests that haven't
// populated them. AssignRole only calls GetUser / GetRole / AssignRole.
func (f *fakeIdentityQueries) ListUsers(context.Context, dbgen.ListUsersParams) ([]dbgen.User, error) {
	panic("ListUsers not implemented in fake")
}
func (f *fakeIdentityQueries) CountUsers(context.Context, uuid.UUID) (int64, error) {
	panic("CountUsers not implemented in fake")
}
func (f *fakeIdentityQueries) CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error) {
	panic("CreateUser not implemented in fake")
}
func (f *fakeIdentityQueries) UpdateUser(context.Context, dbgen.UpdateUserParams) (dbgen.User, error) {
	panic("UpdateUser not implemented in fake")
}
func (f *fakeIdentityQueries) DeactivateUser(context.Context, dbgen.DeactivateUserParams) error {
	panic("DeactivateUser not implemented in fake")
}
func (f *fakeIdentityQueries) ListRoles(context.Context, pgtype.UUID) ([]dbgen.Role, error) {
	panic("ListRoles not implemented in fake")
}
func (f *fakeIdentityQueries) CreateRole(context.Context, dbgen.CreateRoleParams) (dbgen.Role, error) {
	panic("CreateRole not implemented in fake")
}
func (f *fakeIdentityQueries) UpdateRole(context.Context, dbgen.UpdateRoleParams) (dbgen.Role, error) {
	panic("UpdateRole not implemented in fake")
}
func (f *fakeIdentityQueries) DeleteRole(context.Context, dbgen.DeleteRoleParams) error {
	panic("DeleteRole not implemented in fake")
}
func (f *fakeIdentityQueries) RemoveRole(context.Context, dbgen.RemoveRoleParams) error {
	panic("RemoveRole not implemented in fake")
}
func (f *fakeIdentityQueries) ListUserRoleIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	panic("ListUserRoleIDs not implemented in fake")
}

// TestAssignRole_TenantEnforcement covers the three shapes of cross-tenant
// user↔role assignment the migration 000045 trigger and the service-layer
// check are designed to handle:
//
//  1. tenant-A role → tenant-A user       → OK
//  2. tenant-A role → tenant-B user       → ErrCrossTenantRole (400)
//  3. system role (tenant_id=NULL) → any  → OK (system roles are global)
//
// Lookup failures and AssignRole errors bubble up as wrapped errors; we
// verify that AssignRole is NOT called when the tenant check fails.
func TestAssignRole_TenantEnforcement(t *testing.T) {
	t.Parallel()

	tenantA := uuid.New()
	tenantB := uuid.New()

	userA := dbgen.User{ID: uuid.New(), TenantID: tenantA, Username: "alice"}
	userB := dbgen.User{ID: uuid.New(), TenantID: tenantB, Username: "bob"}

	roleA := dbgen.Role{ID: uuid.New(), TenantID: pgtype.UUID{Bytes: tenantA, Valid: true}, Name: "tenant-a-role"}
	roleB := dbgen.Role{ID: uuid.New(), TenantID: pgtype.UUID{Bytes: tenantB, Valid: true}, Name: "tenant-b-role"}
	systemRole := dbgen.Role{ID: uuid.New(), TenantID: pgtype.UUID{Valid: false}, Name: "platform_admin", IsSystem: true}

	tests := []struct {
		name         string
		user         dbgen.User
		role         dbgen.Role
		wantErr      error // sentinel the error must match via errors.Is (nil = expect success)
		wantAssigned bool  // whether queries.AssignRole should have been called
	}{
		{
			name:         "tenant-A role to tenant-A user succeeds",
			user:         userA,
			role:         roleA,
			wantErr:      nil,
			wantAssigned: true,
		},
		{
			name:         "tenant-A role to tenant-B user is rejected",
			user:         userB,
			role:         roleA,
			wantErr:      ErrCrossTenantRole,
			wantAssigned: false,
		},
		{
			name:         "tenant-B role to tenant-A user is rejected",
			user:         userA,
			role:         roleB,
			wantErr:      ErrCrossTenantRole,
			wantAssigned: false,
		},
		{
			name:         "system role to tenant-A user succeeds",
			user:         userA,
			role:         systemRole,
			wantErr:      nil,
			wantAssigned: true,
		},
		{
			name:         "system role to tenant-B user succeeds",
			user:         userB,
			role:         systemRole,
			wantErr:      nil,
			wantAssigned: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange: fresh fake DB with just this user and role populated.
			q := newFakeQueries()
			q.users[tc.user.ID] = tc.user
			q.roles[tc.role.ID] = tc.role
			svc := &Service{queries: q}

			// Act
			err := svc.AssignRole(context.Background(), tc.user.ID, tc.role.ID)

			// Assert: error matches expectation.
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("AssignRole: unexpected error: %v", err)
				}
			} else {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("AssignRole: err = %v, want errors.Is %v", err, tc.wantErr)
				}
			}

			// Assert: AssignRole was (or wasn't) called as expected.
			if tc.wantAssigned && len(q.assignCalls) != 1 {
				t.Errorf("expected AssignRole to be called once, got %d calls", len(q.assignCalls))
			}
			if !tc.wantAssigned && len(q.assignCalls) != 0 {
				t.Errorf("expected AssignRole NOT to be called, got %d calls", len(q.assignCalls))
			}
			if tc.wantAssigned && len(q.assignCalls) == 1 {
				if q.assignCalls[0].UserID != tc.user.ID || q.assignCalls[0].RoleID != tc.role.ID {
					t.Errorf("AssignRole called with wrong params: %+v", q.assignCalls[0])
				}
			}
		})
	}
}

// TestAssignRole_LookupFailures verifies fail-closed behaviour when the
// upstream DB lookups fail — no assignment must happen.
func TestAssignRole_LookupFailures(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	roleID := uuid.New()

	t.Run("user lookup fails", func(t *testing.T) {
		t.Parallel()
		q := newFakeQueries()
		q.getUserErr = errors.New("user pool down")
		svc := &Service{queries: q}

		err := svc.AssignRole(context.Background(), userID, roleID)
		if err == nil {
			t.Fatal("expected error when user lookup fails")
		}
		if len(q.assignCalls) != 0 {
			t.Errorf("AssignRole must not be called when user lookup fails, got %d calls", len(q.assignCalls))
		}
	})

	t.Run("role lookup fails", func(t *testing.T) {
		t.Parallel()
		q := newFakeQueries()
		tenantID := uuid.New()
		q.users[userID] = dbgen.User{ID: userID, TenantID: tenantID}
		q.getRoleErr = errors.New("role pool down")
		svc := &Service{queries: q}

		err := svc.AssignRole(context.Background(), userID, roleID)
		if err == nil {
			t.Fatal("expected error when role lookup fails")
		}
		if len(q.assignCalls) != 0 {
			t.Errorf("AssignRole must not be called when role lookup fails, got %d calls", len(q.assignCalls))
		}
	})
}
