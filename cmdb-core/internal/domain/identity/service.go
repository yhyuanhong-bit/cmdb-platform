package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// ErrCrossTenantRole is returned by AssignRole when the caller attempts to
// assign a tenant-scoped role to a user in a different tenant. The API layer
// translates this to HTTP 400 with code CROSS_TENANT_ROLE.
//
// System roles (roles.tenant_id IS NULL) are intentionally global and do not
// trigger this error.
var ErrCrossTenantRole = errors.New("cross-tenant role assignment")

// ErrUserNotFound is returned when the target user either doesn't exist
// OR exists in a different tenant. We deliberately use the same sentinel
// for both cases so handlers can map it uniformly to HTTP 404 and avoid
// leaking cross-tenant existence to the caller.
var ErrUserNotFound = errors.New("user not found")

// identityQueries is the subset of *dbgen.Queries that Service depends on.
// Defined as an interface so AssignRole's cross-tenant check can be unit
// tested with a lightweight fake. *dbgen.Queries satisfies this interface
// automatically; no adapter is required.
type identityQueries interface {
	ListUsers(ctx context.Context, arg dbgen.ListUsersParams) ([]dbgen.User, error)
	CountUsers(ctx context.Context, tenantID uuid.UUID) (int64, error)
	GetUser(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	GetUserScoped(ctx context.Context, arg dbgen.GetUserScopedParams) (dbgen.User, error)
	CreateUser(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error)
	UpdateUser(ctx context.Context, arg dbgen.UpdateUserParams) (dbgen.User, error)
	DeactivateUser(ctx context.Context, arg dbgen.DeactivateUserParams) error
	ListRoles(ctx context.Context, tenantID pgtype.UUID) ([]dbgen.Role, error)
	GetRole(ctx context.Context, id uuid.UUID) (dbgen.Role, error)
	CreateRole(ctx context.Context, arg dbgen.CreateRoleParams) (dbgen.Role, error)
	UpdateRole(ctx context.Context, arg dbgen.UpdateRoleParams) (dbgen.Role, error)
	DeleteRole(ctx context.Context, arg dbgen.DeleteRoleParams) error
	AssignRole(ctx context.Context, arg dbgen.AssignRoleParams) error
	RemoveRole(ctx context.Context, arg dbgen.RemoveRoleParams) error
	ListUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

// Service provides user and role listing operations.
type Service struct {
	queries identityQueries
}

// NewService creates a new identity Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// ListUsers returns a paginated list of users for the given tenant.
func (s *Service) ListUsers(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.User, int64, error) {
	users, err := s.queries.ListUsers(ctx, dbgen.ListUsersParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	total, err := s.queries.CountUsers(ctx, tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	return users, total, nil
}

// GetUser returns a single user by ID, scoped to the caller's tenant.
// Returns ErrUserNotFound when the user doesn't exist OR belongs to a
// different tenant — handlers must map both cases to HTTP 404 to avoid
// leaking cross-tenant existence.
func (s *Service) GetUser(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.User, error) {
	user, err := s.queries.GetUserScoped(ctx, dbgen.GetUserScopedParams{
		ID: id, TenantID: tenantID,
	})
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// ListRoles returns all roles for the given tenant (including system roles).
func (s *Service) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Role, error) {
	roles, err := s.queries.ListRoles(ctx, pgtype.UUID{Bytes: tenantID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}

// CreateUser creates a new user with a bcrypt-hashed password.
func (s *Service) CreateUser(ctx context.Context, params dbgen.CreateUserParams, plainPassword string) (*dbgen.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	params.PasswordHash = string(hash)

	user, err := s.queries.CreateUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &user, nil
}

// UpdateUser updates an existing user's profile fields. The TenantID
// in params MUST be the caller's tenant — the SQL WHERE pair (id,
// tenant_id) makes a cross-tenant write a 0-row UPDATE which sqlc
// surfaces as pgx.ErrNoRows. We translate that to ErrUserNotFound so
// the handler returns 404 and doesn't leak cross-tenant existence.
func (s *Service) UpdateUser(ctx context.Context, params dbgen.UpdateUserParams) (*dbgen.User, error) {
	user, err := s.queries.UpdateUser(ctx, params)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// CreateRole creates a new custom role.
func (s *Service) CreateRole(ctx context.Context, params dbgen.CreateRoleParams) (*dbgen.Role, error) {
	role, err := s.queries.CreateRole(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return &role, nil
}

// UpdateRole updates a non-system role by ID.
func (s *Service) UpdateRole(ctx context.Context, params dbgen.UpdateRoleParams) (*dbgen.Role, error) {
	role, err := s.queries.UpdateRole(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	return &role, nil
}

// AssignRole assigns a role to a user, enforcing same-tenant isolation.
//
// Three layers of defense:
//  1. Tenant-scoped user lookup (NEW): GetUserScoped fails closed when
//     the target user doesn't belong to the caller's tenant. Without this
//     check, a tenant A admin could assign a system role to a tenant B
//     user — privilege escalation across tenants.
//  2. Application-layer role check: fetch roles.tenant_id, return
//     ErrCrossTenantRole when the role is tenant-scoped and differs
//     from the user's tenant. System roles bypass this and are
//     assignable to any user (within the caller's own tenant).
//  3. Database trigger trg_user_roles_tenant_check (migration 000045)
//     which re-validates on INSERT/UPDATE — last line of defense if
//     application checks are bypassed.
func (s *Service) AssignRole(ctx context.Context, tenantID, userID, roleID uuid.UUID) error {
	user, err := s.queries.GetUserScoped(ctx, dbgen.GetUserScopedParams{
		ID: userID, TenantID: tenantID,
	})
	if err != nil {
		return ErrUserNotFound
	}
	role, err := s.queries.GetRole(ctx, roleID)
	if err != nil {
		return fmt.Errorf("assign role: lookup role: %w", err)
	}

	// System roles carry a NULL tenant_id and may be assigned to any user.
	// Tenant-scoped roles must match the user's tenant exactly.
	if role.TenantID.Valid {
		roleTenant := uuid.UUID(role.TenantID.Bytes)
		if roleTenant != user.TenantID {
			return fmt.Errorf("%w: user_tenant=%s role_tenant=%s",
				ErrCrossTenantRole, user.TenantID, roleTenant)
		}
	}

	if err := s.queries.AssignRole(ctx, dbgen.AssignRoleParams{UserID: userID, RoleID: roleID}); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a role from a user. Validates the user belongs to
// the caller's tenant first — without this check, a tenant A admin
// could strip roles from a tenant B user (DoS / lockout attack).
func (s *Service) RemoveRole(ctx context.Context, tenantID, userID, roleID uuid.UUID) error {
	if _, err := s.queries.GetUserScoped(ctx, dbgen.GetUserScopedParams{
		ID: userID, TenantID: tenantID,
	}); err != nil {
		return ErrUserNotFound
	}
	if err := s.queries.RemoveRole(ctx, dbgen.RemoveRoleParams{UserID: userID, RoleID: roleID}); err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	return nil
}

// ListUserRoleIDs returns the role IDs assigned to a user. Scoped to the
// caller's tenant — without this, tenant A could enumerate tenant B's
// user-role assignments.
func (s *Service) ListUserRoleIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error) {
	if _, err := s.queries.GetUserScoped(ctx, dbgen.GetUserScopedParams{
		ID: userID, TenantID: tenantID,
	}); err != nil {
		return nil, ErrUserNotFound
	}
	ids, err := s.queries.ListUserRoleIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user role IDs: %w", err)
	}
	return ids, nil
}

// Deactivate soft-deletes a user by setting status to 'deleted'.
func (s *Service) Deactivate(ctx context.Context, tenantID, userID uuid.UUID) error {
	err := s.queries.DeactivateUser(ctx, dbgen.DeactivateUserParams{ID: userID, TenantID: tenantID})
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	return nil
}

// DeleteRole deletes a non-system role by ID, scoped to the given tenant.
func (s *Service) DeleteRole(ctx context.Context, tenantID, id uuid.UUID) error {
	err := s.queries.DeleteRole(ctx, dbgen.DeleteRoleParams{ID: id, TenantID: pgtype.UUID{Bytes: tenantID, Valid: true}})
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}
