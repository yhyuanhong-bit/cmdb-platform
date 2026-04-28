package api

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/audit"
	"github.com/google/uuid"
)

// identityService is the narrow interface the api package depends on for
// user and role management. It matches a subset of *identity.Service so
// handlers can be unit-tested with a mock.
type identityService interface {
	ListUsers(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]dbgen.User, int64, error)
	GetUser(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.User, error)
	CreateUser(ctx context.Context, params dbgen.CreateUserParams, plainPassword string) (*dbgen.User, error)
	UpdateUser(ctx context.Context, params dbgen.UpdateUserParams) (*dbgen.User, error)
	ListRoles(ctx context.Context, tenantID uuid.UUID) ([]dbgen.Role, error)
	CreateRole(ctx context.Context, params dbgen.CreateRoleParams) (*dbgen.Role, error)
	UpdateRole(ctx context.Context, params dbgen.UpdateRoleParams) (*dbgen.Role, error)
	DeleteRole(ctx context.Context, tenantID, id uuid.UUID) error
	AssignRole(ctx context.Context, tenantID, userID, roleID uuid.UUID) error
	RemoveRole(ctx context.Context, tenantID, userID, roleID uuid.UUID) error
	ListUserRoleIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error)
	Deactivate(ctx context.Context, tenantID, userID uuid.UUID) error
}

// auditService is the narrow interface for audit event read/write from handlers.
type auditService interface {
	Record(
		ctx context.Context,
		tenantID uuid.UUID,
		action, module, targetType string,
		targetID uuid.UUID,
		operatorType audit.OperatorType,
		operatorID *uuid.UUID,
		diff map[string]any,
		source string,
	) error
	Query(ctx context.Context, tenantID uuid.UUID, module, targetType *string, targetID *uuid.UUID, limit, offset int32) ([]dbgen.AuditEvent, int64, error)
}
