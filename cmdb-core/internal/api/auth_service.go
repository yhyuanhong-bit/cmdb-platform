package api

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/domain/identity"
	"github.com/google/uuid"
)

// authService is the narrow interface the api package depends on for
// authentication. It matches a subset of *identity.AuthService so the
// concrete service can be swapped with a test double in handler tests.
type authService interface {
	Login(ctx context.Context, req identity.LoginRequest) (*identity.TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*identity.TokenResponse, error)
	GetCurrentUser(ctx context.Context, userID string) (*identity.CurrentUser, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}
