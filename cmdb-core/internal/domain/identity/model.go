package identity

import "github.com/google/uuid"

// LoginRequest is the payload for username/password authentication.
//
// TenantSlug is optional. When provided, the auth service disambiguates the
// user by (tenant_id, username); when empty, it falls back to the legacy
// globally-unique-username lookup and logs a deprecation warning. Ambiguous
// usernames on the fallback path fail closed — the caller must retry with an
// explicit tenant_slug.
type LoginRequest struct {
	TenantSlug string `json:"tenant_slug,omitempty"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	ClientIP   string `json:"-"`
	UserAgent  string `json:"-"`
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
