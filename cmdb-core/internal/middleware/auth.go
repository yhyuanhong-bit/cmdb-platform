package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JWTClaims holds the custom claims embedded in a JWT token.
//
// ID (the standard "jti" claim) uniquely identifies each issued token; it is
// used to revoke individual access tokens via Redis blacklist on logout.
// IssuedAt (the standard "iat" claim) records when the token was minted; it
// lets the server reject tokens issued before a user's last password change.
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	TenantID  string `json:"tenant_id"`
	DeptID    string `json:"dept_id,omitempty"`
	ID        string `json:"jti,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

// RevocationChecker is implemented by auth.Blacklist (and test doubles).
// Defined here to avoid an import cycle between middleware and auth packages.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// PasswordChangeChecker reports the timestamp of a user's most recent
// password change. Tokens issued before that moment are rejected.
type PasswordChangeChecker interface {
	PasswordChangedAt(ctx context.Context, userID, tenantID string) (time.Time, error)
}

// AuthOption configures optional behaviour of the Auth middleware.
type AuthOption func(*authConfig)

type authConfig struct {
	blacklist  RevocationChecker
	pwdChecker PasswordChangeChecker
}

// WithBlacklist installs a revocation checker. When set, the middleware
// rejects tokens whose jti has been revoked.
func WithBlacklist(b RevocationChecker) AuthOption {
	return func(c *authConfig) { c.blacklist = b }
}

// WithPasswordChangeChecker installs a password-rotation checker. When set,
// the middleware rejects tokens issued before the user's last password
// change.
func WithPasswordChangeChecker(p PasswordChangeChecker) AuthOption {
	return func(c *authConfig) { c.pwdChecker = p }
}

// Auth returns a Gin middleware that validates JWT Bearer tokens using HMAC-SHA256.
// Optional checkers (blacklist, password-change) layer on top: when nil, those
// checks are skipped, which keeps existing tests and dev setups working without
// a Redis/DB connection.
func Auth(secret string, opts ...AuthOption) gin.HandlerFunc {
	cfg := &authConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Err(c, 401, "INVALID_TOKEN", "missing or malformed authorization header")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := validateJWT(token, secret)
		if err != nil {
			response.Err(c, 401, "INVALID_TOKEN", err.Error())
			c.Abort()
			return
		}

		if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
			response.Err(c, 401, "INVALID_TOKEN", "token has expired")
			c.Abort()
			return
		}

		// Revocation check via Redis blacklist. A Redis outage fails OPEN with
		// a loud warning so dev environments stay usable; production should
		// alert on this log line.
		if cfg.blacklist != nil && claims.ID != "" {
			revoked, rerr := cfg.blacklist.IsRevoked(c.Request.Context(), claims.ID)
			switch {
			case rerr != nil:
				zap.L().Warn("auth middleware: blacklist unavailable, failing open",
					zap.String("jti", claims.ID),
					zap.Error(rerr))
			case revoked:
				response.Err(c, 401, "TOKEN_REVOKED", "token has been revoked")
				c.Abort()
				return
			}
		}

		// Password-rotation check. Any token minted before the user last
		// rotated their password is rejected.
		if cfg.pwdChecker != nil && claims.IssuedAt > 0 {
			pwdChangedAt, perr := cfg.pwdChecker.PasswordChangedAt(c.Request.Context(), claims.UserID, claims.TenantID)
			switch {
			case perr != nil:
				zap.L().Warn("auth middleware: password-change check failed, failing open",
					zap.String("user_id", claims.UserID),
					zap.Error(perr))
			case !pwdChangedAt.IsZero() && claims.IssuedAt < pwdChangedAt.Unix():
				response.Err(c, 401, "TOKEN_OUTDATED", "token was issued before last password change")
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("tenant_id", claims.TenantID)
		if claims.DeptID != "" {
			c.Set("dept_id", claims.DeptID)
		}
		if claims.ID != "" {
			c.Set("jti", claims.ID)
		}
		if claims.ExpiresAt > 0 {
			c.Set("token_exp", claims.ExpiresAt)
		}

		c.Next()
	}
}

// validateJWT splits the token on ".", verifies the HMAC-SHA256 signature, and
// decodes the base64url-encoded payload into JWTClaims.
func validateJWT(token, secret string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errInvalidToken
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	if !hmac.Equal(signature, expected) {
		return nil, errInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errInvalidToken
	}

	return &claims, nil
}

// GenerateJWT creates a signed JWT string from the given claims using HMAC-SHA256.
func GenerateJWT(claims JWTClaims, secret string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

var errInvalidToken = &tokenError{msg: "invalid token"}

type tokenError struct {
	msg string
}

func (e *tokenError) Error() string { return e.msg }
