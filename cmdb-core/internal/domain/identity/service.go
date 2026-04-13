package identity

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// Service provides user and role listing operations.
type Service struct {
	queries *dbgen.Queries
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

// GetUser returns a single user by ID.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*dbgen.User, error) {
	user, err := s.queries.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
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

// UpdateUser updates an existing user's profile fields.
func (s *Service) UpdateUser(ctx context.Context, params dbgen.UpdateUserParams) (*dbgen.User, error) {
	user, err := s.queries.UpdateUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
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

// AssignRole assigns a role to a user.
func (s *Service) AssignRole(ctx context.Context, userID, roleID uuid.UUID) error {
	err := s.queries.AssignRole(ctx, dbgen.AssignRoleParams{UserID: userID, RoleID: roleID})
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a role from a user.
func (s *Service) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	err := s.queries.RemoveRole(ctx, dbgen.RemoveRoleParams{UserID: userID, RoleID: roleID})
	if err != nil {
		return fmt.Errorf("remove role: %w", err)
	}
	return nil
}

// ListUserRoleIDs returns the role IDs assigned to a user.
func (s *Service) ListUserRoleIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
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
