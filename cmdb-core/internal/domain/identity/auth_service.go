package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	refreshPrefix   = "refresh:"
)

// AuthService handles authentication and token management.
type AuthService struct {
	queries   *dbgen.Queries
	redis     *redis.Client
	jwtSecret string
}

// NewAuthService creates a new AuthService.
func NewAuthService(queries *dbgen.Queries, rdb *redis.Client, jwtSecret string) *AuthService {
	return &AuthService{
		queries:   queries,
		redis:     rdb,
		jwtSecret: jwtSecret,
	}
}

// Login authenticates a user by username and password and returns tokens.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	if user.Status != "active" {
		return nil, errors.New("account is not active")
	}

	return s.issueTokens(ctx, user)
}

// Refresh validates a refresh token and issues a new token pair (rotation).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	key := refreshPrefix + refreshToken
	userIDStr, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Delete the used refresh token (rotation)
	s.redis.Del(ctx, key)

	userID := parseUUID(userIDStr)
	user, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return s.issueTokens(ctx, user)
}

// GetCurrentUser returns the authenticated user with merged role permissions.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*CurrentUser, error) {
	uid := parseUUID(userID)
	user, err := s.queries.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	roles, err := s.queries.ListUserRoles(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list user roles: %w", err)
	}

	permissions := make(map[string][]string)
	for _, role := range roles {
		if role.Permissions == nil {
			continue
		}
		var rolePerm map[string][]string
		if err := json.Unmarshal(role.Permissions, &rolePerm); err != nil {
			continue
		}
		for resource, actions := range rolePerm {
			permissions[resource] = appendUnique(permissions[resource], actions...)
		}
	}

	return &CurrentUser{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Permissions: permissions,
	}, nil
}

// issueTokens creates a JWT access token and a random refresh token stored in Redis.
func (s *AuthService) issueTokens(ctx context.Context, user dbgen.User) (*TokenResponse, error) {
	deptID := ""
	if user.DeptID.Valid {
		deptID = uuid.UUID(user.DeptID.Bytes).String()
	}

	claims := middleware.JWTClaims{
		UserID:    user.ID.String(),
		Username:  user.Username,
		TenantID:  user.TenantID.String(),
		DeptID:    deptID,
		ExpiresAt: time.Now().Add(accessTokenTTL).Unix(),
	}

	accessToken, err := middleware.GenerateJWT(claims, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	key := refreshPrefix + refreshToken
	if err := s.redis.Set(ctx, key, user.ID.String(), refreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
	}, nil
}

// generateSecureToken produces a 32-byte cryptographically random token encoded as base64url.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// parseUUID converts a string to uuid.UUID, returning uuid.Nil on failure.
func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// appendUnique appends values to a slice, skipping duplicates.
func appendUnique(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, v := range existing {
		seen[v] = struct{}{}
	}
	for _, v := range values {
		if _, ok := seen[v]; !ok {
			existing = append(existing, v)
			seen[v] = struct{}{}
		}
	}
	return existing
}
