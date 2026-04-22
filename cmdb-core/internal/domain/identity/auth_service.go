package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	refreshPrefix   = "refresh:"
	// refreshUserIndexPrefix keys a Redis Set of refresh token keys owned by
	// a given user, so that logout and password-change flows can invalidate
	// every outstanding refresh token in one call.
	refreshUserIndexPrefix = "refresh_index:user:"
	// pwdChangedCachePrefix caches users.password_changed_at so that every
	// authenticated request does not hit the DB. Bust on password change.
	pwdChangedCachePrefix = "pwd_changed:"
	pwdChangedCacheTTL    = 60 * time.Second
	// systemUserSource is the users.source value for the per-tenant sentinel
	// seeded by migration 000052. Login and any UI user-picker path MUST
	// reject this value so the FK-safe sentinel cannot authenticate or be
	// assigned work. Mirrors the literal in the migration — kept here
	// instead of importing platform/identity to avoid pulling the whole
	// resolver into every auth-service consumer.
	systemUserSource = "system"
)

// Blacklist is the subset of *auth.Blacklist needed by AuthService. A narrow
// interface keeps the domain package decoupled from the auth package's
// concrete Redis client.
type Blacklist interface {
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
}

// AuthService handles authentication and token management.
type AuthService struct {
	queries   *dbgen.Queries
	redis     *redis.Client
	jwtSecret string
	pool      *pgxpool.Pool
	blacklist Blacklist
}

// NewAuthService creates a new AuthService.
func NewAuthService(queries *dbgen.Queries, rdb *redis.Client, jwtSecret string, pool *pgxpool.Pool) *AuthService {
	return &AuthService{
		queries:   queries,
		redis:     rdb,
		jwtSecret: jwtSecret,
		pool:      pool,
	}
}

// WithBlacklist installs a token blacklist. Logout uses it to revoke the
// current access token's jti. Safe to call with nil to disable (tests).
func (s *AuthService) WithBlacklist(b Blacklist) *AuthService {
	s.blacklist = b
	return s
}

// Login authenticates a user and returns tokens.
//
// The lookup path depends on LoginRequest.TenantSlug:
//
//   - When a non-empty slug is supplied, the tenant is resolved by slug and
//     the user is looked up via the (tenant_id, username) unique index. An
//     unknown slug, a missing user, a wrong password, or an inactive account
//     all fail with the same generic message to avoid leaking which tenants
//     and usernames exist.
//
//   - When the slug is empty, the legacy global-username path is used. That
//     path is deprecated (a warning is logged on every use) and fails closed
//     if the username is not globally unique — the caller must retry with a
//     tenant_slug.
//
// See Phase 1.3 of the remediation roadmap for the full rationale and the
// users_tenant_username_unique index that backs this behaviour.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.resolveLoginUser(ctx, req)
	if err != nil {
		return nil, err
	}

	// The per-tenant 'system' user (migration 000052) exists only to satisfy
	// FK constraints on workflow-triggered writes — it has no real password
	// (seeded with '!' as password_hash) and must never authenticate. Fail
	// closed *before* bcrypt so a lucky password collision cannot succeed
	// and so we don't burn a bcrypt compare on a sentinel account. The
	// generic error matches the wrong-password path to avoid leaking which
	// usernames are sentinels.
	if user.Source == systemUserSource {
		return nil, errors.New("invalid username or password")
	}

	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); bcryptErr != nil {
		return nil, errors.New("invalid username or password")
	}

	if user.Status != "active" {
		return nil, errors.New("account is not active")
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	s.recordSession(ctx, user.ID, req.ClientIP, req.UserAgent)
	return tokens, nil
}

// resolveLoginUser picks the user to authenticate given a LoginRequest.
// Returns a generic "invalid username or password" error on every failure
// mode — slug missing, user missing, and ambiguous username all look the
// same to the caller to avoid information leaks.
func (s *AuthService) resolveLoginUser(ctx context.Context, req LoginRequest) (dbgen.User, error) {
	genericErr := errors.New("invalid username or password")

	if req.TenantSlug != "" {
		tenant, err := s.queries.GetTenantBySlug(ctx, req.TenantSlug)
		if err != nil {
			return dbgen.User{}, genericErr
		}
		user, err := s.queries.GetUserByTenantAndUsername(ctx, dbgen.GetUserByTenantAndUsernameParams{
			TenantID: tenant.ID,
			Username: req.Username,
		})
		if err != nil {
			return dbgen.User{}, genericErr
		}
		return user, nil
	}

	// Legacy fallback: no tenant_slug supplied. Phase 1.3 deprecates this
	// path because (tenant_id, username) is the new uniqueness contract.
	// Keep working only when the username happens to be globally unique
	// so existing single-tenant deployments don't break; fail closed as
	// soon as the column is genuinely ambiguous.
	candidates, err := s.queries.ListUsersByUsername(ctx, req.Username)
	if err != nil {
		return dbgen.User{}, genericErr
	}
	if len(candidates) == 0 {
		return dbgen.User{}, genericErr
	}
	if len(candidates) > 1 {
		zap.L().Warn("login: ambiguous username without tenant_slug",
			zap.String("username", req.Username),
			zap.Int("matches", len(candidates)))
		return dbgen.User{}, genericErr
	}
	zap.L().Warn("login: tenant_slug missing; relying on deprecated global-username fallback",
		zap.String("username", req.Username))
	return candidates[0], nil
}

// parseUserAgent extracts device type and browser from a User-Agent string.
func parseUserAgent(ua string) (deviceType, browser string) {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	case strings.Contains(lower, "edg"):
		browser = "Edge"
	default:
		browser = "unknown"
	}
	switch {
	case strings.Contains(lower, "mobile") || strings.Contains(lower, "android"):
		deviceType = "mobile"
	case strings.Contains(lower, "tablet") || strings.Contains(lower, "ipad"):
		deviceType = "tablet"
	default:
		deviceType = "desktop"
	}
	return
}

// recordSession persists the login session and updates the user's last login info.
func (s *AuthService) recordSession(ctx context.Context, userID uuid.UUID, clientIP, userAgent string) {
	if s.pool == nil {
		return
	}
	deviceType, browser := parseUserAgent(userAgent)
	q := dbgen.New(s.pool)
	if err := q.ClearCurrentUserSessions(ctx, userID); err != nil {
		zap.L().Warn("recordSession: failed to clear current sessions", zap.Error(err))
	}
	if err := q.InsertUserSession(ctx, dbgen.InsertUserSessionParams{
		UserID:     userID,
		IpAddress:  pgtype.Text{String: clientIP, Valid: clientIP != ""},
		UserAgent:  pgtype.Text{String: userAgent, Valid: userAgent != ""},
		DeviceType: pgtype.Text{String: deviceType, Valid: deviceType != ""},
		Browser:    pgtype.Text{String: browser, Valid: browser != ""},
	}); err != nil {
		zap.L().Warn("recordSession: failed to insert session", zap.Error(err))
	}
	if _, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at = now(), last_login_ip = $1 WHERE id = $2`, clientIP, userID); err != nil {
		zap.L().Warn("recordSession: failed to update last login", zap.Error(err))
	}
}

// ChangePassword validates the current password and sets a new one.
//
// On success it also:
//  1. Bumps users.password_changed_at — tokens issued before this point are
//     rejected by the auth middleware.
//  2. Revokes every outstanding refresh token for the user.
//
// The access-token revocation is handled implicitly by the password_changed_at
// check (no blacklist write per-jti is needed).
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.queries.GetUser(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); bcryptErr != nil {
		return errors.New("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if s.pool != nil {
		if _, err = s.pool.Exec(ctx,
			`UPDATE users SET password_hash = $1, password_changed_at = now(), updated_at = now() WHERE id = $2`,
			string(hash), userID); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
	}
	// Best-effort refresh-token revocation and password-change cache bust.
	if s.redis != nil {
		s.revokeAllRefreshTokens(ctx, userID)
		// Bust any cached password_changed_at so the middleware picks up the
		// new value immediately rather than after the cache TTL.
		_ = s.redis.Del(ctx, pwdChangedCachePrefix+userID.String()).Err()
	}
	return nil
}

// Logout revokes the access token identified by jti (via Redis blacklist)
// and deletes every outstanding refresh token for the user.
func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID, jti string, tokenExpiresAt time.Time) error {
	if s.blacklist != nil && jti != "" {
		if err := s.blacklist.Revoke(ctx, jti, tokenExpiresAt); err != nil {
			return fmt.Errorf("failed to revoke access token: %w", err)
		}
	}
	if s.redis != nil && userID != uuid.Nil {
		s.revokeAllRefreshTokens(ctx, userID)
	}
	return nil
}

// Refresh validates a refresh token and issues a new token pair (rotation).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	key := refreshPrefix + refreshToken
	userIDStr, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Delete the used refresh token (rotation) and its index entry.
	s.redis.Del(ctx, key)
	s.redis.SRem(ctx, refreshUserIndexPrefix+userIDStr, key)

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
// Each token carries a fresh jti (uuid) and iat so individual tokens can be
// revoked via the blacklist.
func (s *AuthService) issueTokens(ctx context.Context, user dbgen.User) (*TokenResponse, error) {
	deptID := ""
	if user.DeptID.Valid {
		deptID = uuid.UUID(user.DeptID.Bytes).String()
	}

	now := time.Now()
	claims := middleware.JWTClaims{
		UserID:    user.ID.String(),
		Username:  user.Username,
		TenantID:  user.TenantID.String(),
		DeptID:    deptID,
		ID:        uuid.New().String(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(accessTokenTTL).Unix(),
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

	// Best-effort per-user index so logout / password-change can wipe every
	// outstanding refresh token. A failure here should not fail login.
	indexKey := refreshUserIndexPrefix + user.ID.String()
	if err := s.redis.SAdd(ctx, indexKey, key).Err(); err != nil {
		zap.L().Warn("issueTokens: failed to index refresh token for user",
			zap.String("user_id", user.ID.String()), zap.Error(err))
	} else {
		// Keep the index from growing forever in the presence of orphaned
		// keys: expire the set itself after one refresh-token lifetime past
		// now. Cleanup on logout / password-change still happens explicitly.
		_ = s.redis.Expire(ctx, indexKey, refreshTokenTTL).Err()
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
	}, nil
}

// revokeAllRefreshTokens deletes every refresh token registered for a user
// plus the index itself. Safe to call even when no tokens exist.
func (s *AuthService) revokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) {
	indexKey := refreshUserIndexPrefix + userID.String()
	keys, err := s.redis.SMembers(ctx, indexKey).Result()
	if err != nil {
		zap.L().Warn("revokeAllRefreshTokens: SMembers failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		return
	}
	if len(keys) > 0 {
		if err := s.redis.Del(ctx, keys...).Err(); err != nil {
			zap.L().Warn("revokeAllRefreshTokens: Del tokens failed",
				zap.String("user_id", userID.String()), zap.Error(err))
		}
	}
	if err := s.redis.Del(ctx, indexKey).Err(); err != nil {
		zap.L().Warn("revokeAllRefreshTokens: Del index failed",
			zap.String("user_id", userID.String()), zap.Error(err))
	}
}

// PasswordChangedAt returns the password_changed_at timestamp for a user.
// Values are cached in Redis for 60s to avoid a DB round-trip per request.
// A missing cache entry or failed DB query returns (zero time, nil) which
// the middleware treats as "no change timestamp available" (fail-open).
//
// Implements middleware.PasswordChangeChecker.
func (s *AuthService) PasswordChangedAt(ctx context.Context, userIDStr string) (time.Time, error) {
	if s.pool == nil {
		return time.Time{}, nil
	}
	userID := parseUUID(userIDStr)
	if userID == uuid.Nil {
		return time.Time{}, nil
	}

	if s.redis != nil {
		if val, err := s.redis.Get(ctx, pwdChangedCachePrefix+userIDStr).Result(); err == nil {
			if ts, perr := strconv.ParseInt(val, 10, 64); perr == nil {
				return time.Unix(ts, 0), nil
			}
		}
	}

	var changedAt time.Time
	row := s.pool.QueryRow(ctx, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	if err := row.Scan(&changedAt); err != nil {
		return time.Time{}, fmt.Errorf("fetch password_changed_at: %w", err)
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, pwdChangedCachePrefix+userIDStr,
			strconv.FormatInt(changedAt.Unix(), 10), pwdChangedCacheTTL).Err()
	}
	return changedAt, nil
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
