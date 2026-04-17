package api

import (
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
