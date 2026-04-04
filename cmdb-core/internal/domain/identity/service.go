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

// DeleteRole deletes a non-system role by ID.
func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteRole(ctx, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}
