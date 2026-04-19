package api

import (
	"net/http"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Auth endpoints
// ---------------------------------------------------------------------------

// Login authenticates a user and returns a token pair.
// (POST /auth/login)
func (s *APIServer) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tokens, err := s.authSvc.Login(c.Request.Context(), identity.LoginRequest{
		Username:  req.Username,
		Password:  req.Password,
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// RefreshToken issues a new token pair using a refresh token.
// (POST /auth/refresh)
func (s *APIServer) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tokens, err := s.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// Logout revokes the current access token (via Redis blacklist) and clears
// every outstanding refresh token for the user.
// (POST /auth/logout)
//
// This endpoint is registered manually in cmd/server/main.go rather than in
// the generated ServerInterface because the OpenAPI spec regeneration is out
// of scope for this change; the generated router is augmented via a direct
// route registration alongside other "Track B" endpoints.
func (s *APIServer) Logout(c *gin.Context) {
	userID := userIDFromContext(c)
	jti := c.GetString("jti")

	var expiresAt time.Time
	if v, ok := c.Get("token_exp"); ok {
		if ts, ok := v.(int64); ok && ts > 0 {
			expiresAt = time.Unix(ts, 0)
		}
	}
	// Fall back to a safe upper bound (15m) when the middleware did not
	// populate token_exp. Still correct because the token can't outlive its
	// signature-verified exp anyway.
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(15 * time.Minute)
	}

	if err := s.authSvc.Logout(c.Request.Context(), userID, jti, expiresAt); err != nil {
		response.InternalError(c, "failed to logout")
		return
	}
	c.AbortWithStatus(http.StatusNoContent)
}

// GetCurrentUser returns the authenticated user with merged permissions.
// (GET /auth/me)
func (s *APIServer) GetCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	cu, err := s.authSvc.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.Unauthorized(c, "failed to get current user")
		return
	}
	response.OK(c, CurrentUser{
		Id:          cu.ID,
		Username:    cu.Username,
		DisplayName: cu.DisplayName,
		Email:       cu.Email,
		Permissions: cu.Permissions,
	})
}
