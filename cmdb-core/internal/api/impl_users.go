package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Identity endpoints
// ---------------------------------------------------------------------------

// ListUsers returns a paginated list of users.
// (GET /users)
func (s *APIServer) ListUsers(c *gin.Context, params ListUsersParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	users, total, err := s.identitySvc.ListUsers(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}
	response.OKList(c, convertSlice(users, toAPIUser), page, pageSize, int(total))
}

// GetUser returns a single user by ID, scoped to the caller's tenant.
// (GET /users/{id})
func (s *APIServer) GetUser(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	user, err := s.identitySvc.GetUser(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, toAPIUser(*user))
}

// CreateUser creates a new user.
// (POST /users)
func (s *APIServer) CreateUser(c *gin.Context) {
	var req CreateUserJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	status := "active"
	if req.Status != nil {
		status = *req.Status
	}
	source := "local"
	if req.Source != nil {
		source = *req.Source
	}

	params := dbgen.CreateUserParams{
		TenantID:    tenantID,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       pgtype.Text{String: "", Valid: false},
		Status:      status,
		Source:      source,
	}
	if req.Phone != nil {
		params.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}

	user, err := s.identitySvc.CreateUser(c.Request.Context(), params, req.Password)
	if err != nil {
		response.InternalError(c, "failed to create user")
		return
	}
	s.recordAudit(c, "user.created", "identity", "user", user.ID, map[string]any{
		"username": user.Username,
	})
	response.Created(c, toAPIUser(*user))
}

// UpdateUser updates an existing user. Tenant-scoped — the SQL UPDATE
// pairs id with tenant_id so a cross-tenant write becomes a 0-row
// no-op which the service translates to ErrUserNotFound (404).
// (PUT /users/{id})
func (s *APIServer) UpdateUser(c *gin.Context, id IdPath) {
	var req UpdateUserJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateUserParams{
		ID:       uuid.UUID(id),
		TenantID: tenantIDFromContext(c),
	}
	if req.DisplayName != nil {
		params.DisplayName = pgtype.Text{String: *req.DisplayName, Valid: true}
	}
	if req.Email != nil {
		params.Email = pgtype.Text{String: *req.Email, Valid: true}
	}
	if req.Phone != nil {
		params.Phone = pgtype.Text{String: *req.Phone, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}

	user, err := s.identitySvc.UpdateUser(c.Request.Context(), params)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	s.recordAudit(c, "user.updated", "identity", "user", user.ID, map[string]any{
		"username": user.Username,
	})
	response.OK(c, toAPIUser(*user))
}

// ListRoles returns all roles for the tenant.
// (GET /roles)
func (s *APIServer) ListRoles(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	roles, err := s.identitySvc.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list roles")
		return
	}
	response.OK(c, convertSlice(roles, toAPIRole))
}

// CreateRole creates a new custom role.
// (POST /roles)
func (s *APIServer) CreateRole(c *gin.Context) {
	var req CreateRoleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var permJSON json.RawMessage
	if req.Permissions != nil {
		b, _ := json.Marshal(*req.Permissions)
		permJSON = b
	} else {
		permJSON = json.RawMessage(`{}`)
	}

	params := dbgen.CreateRoleParams{
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Name:        req.Name,
		Permissions: permJSON,
		IsSystem:    false,
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	role, err := s.identitySvc.CreateRole(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create role")
		return
	}
	s.recordAudit(c, "role.created", "identity", "role", role.ID, map[string]any{
		"name": role.Name,
	})
	response.Created(c, toAPIRole(*role))
}

// UpdateRole updates a non-system role.
// (PUT /roles/{id})
func (s *APIServer) UpdateRole(c *gin.Context, id IdPath) {
	var req UpdateRoleJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateRoleParams{
		ID:       uuid.UUID(id),
		TenantID: pgtype.UUID{Bytes: tenantIDFromContext(c), Valid: true},
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Permissions != nil {
		b, _ := json.Marshal(*req.Permissions)
		params.Permissions = b
	}

	role, err := s.identitySvc.UpdateRole(c.Request.Context(), params)
	if err != nil {
		response.NotFound(c, "role not found or is a system role")
		return
	}
	s.recordAudit(c, "role.updated", "identity", "role", role.ID, map[string]any{
		"name": role.Name,
	})
	response.OK(c, toAPIRole(*role))
}

// DeleteRole deletes a non-system role.
// (DELETE /roles/{id})
func (s *APIServer) DeleteRole(c *gin.Context, id IdPath) {
	err := s.identitySvc.DeleteRole(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		// Pre-W6.3 returned 204 silently on cross-tenant or system-role hits;
		// service now surfaces ErrRoleNotFound on 0 rows. Either way uniform 404.
		response.NotFound(c, "role not found or is a system role")
		return
	}
	s.recordAudit(c, "role.deleted", "identity", "role", uuid.UUID(id), nil)
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Role Assignment + User Deletion endpoints
// ---------------------------------------------------------------------------

// AssignRoleToUser assigns a role to a user.
// (POST /users/{id}/roles)
func (s *APIServer) AssignRoleToUser(c *gin.Context, id IdPath) {
	userID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)
	var req AssignRoleToUserJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "role_id is required")
		return
	}
	roleID := uuid.UUID(req.RoleId)
	if err := s.identitySvc.AssignRole(c.Request.Context(), tenantID, userID, roleID); err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		if errors.Is(err, identity.ErrCrossTenantRole) {
			response.Err(c, http.StatusBadRequest, "CROSS_TENANT_ROLE",
				"role belongs to a different tenant than the user")
			return
		}
		response.InternalError(c, "failed to assign role")
		return
	}
	s.recordAudit(c, "role.assigned", "identity", "user", userID, map[string]any{
		"role_id": roleID.String(),
	})
	response.OK(c, gin.H{"assigned": true})
}

// RemoveRoleFromUser removes a role from a user.
// (DELETE /users/{id}/roles/{roleId})
func (s *APIServer) RemoveRoleFromUser(c *gin.Context, id IdPath, roleId openapi_types.UUID) {
	userID := uuid.UUID(id)
	roleID := uuid.UUID(roleId)
	tenantID := tenantIDFromContext(c)
	if err := s.identitySvc.RemoveRole(c.Request.Context(), tenantID, userID, roleID); err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "failed to remove role")
		return
	}
	s.recordAudit(c, "role.removed", "identity", "user", userID, map[string]any{
		"role_id": roleID.String(),
	})
	c.Status(204)
}

// ListUserRoles returns roles assigned to a user.
// (GET /users/{id}/roles)
func (s *APIServer) ListUserRoles(c *gin.Context, id IdPath) {
	userID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)
	roleIDs, err := s.identitySvc.ListUserRoleIDs(c.Request.Context(), tenantID, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "failed to list user roles")
		return
	}
	response.OK(c, roleIDs)
}

// DeleteUser soft-deletes (deactivates) a user.
// (DELETE /users/{id})
func (s *APIServer) DeleteUser(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := uuid.UUID(id)
	if err := s.identitySvc.Deactivate(c.Request.Context(), tenantID, userID); err != nil {
		response.InternalError(c, "failed to delete user")
		return
	}
	s.recordAudit(c, "user.deleted", "identity", "user", userID, nil)
	c.Status(204)
}
