package identity

import "github.com/google/uuid"

// LoginRequest is the payload for username/password authentication.
type LoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

// TokenResponse is returned after successful login or token refresh.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshRequest is the payload for refreshing an access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// CurrentUser represents the authenticated user with merged permissions.
type CurrentUser struct {
	ID          uuid.UUID            `json:"id"`
	Username    string               `json:"username"`
	DisplayName string               `json:"display_name"`
	Email       string               `json:"email"`
	Permissions map[string][]string  `json:"permissions"`
}
