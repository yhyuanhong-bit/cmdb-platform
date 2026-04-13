package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// JWTClaims holds the custom claims embedded in a JWT token.
type JWTClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	TenantID  string `json:"tenant_id"`
	DeptID    string `json:"dept_id,omitempty"`
	ExpiresAt int64  `json:"exp"`
}

// Auth returns a Gin middleware that validates JWT Bearer tokens using HMAC-SHA256.
func Auth(secret string) gin.HandlerFunc {
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

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("tenant_id", claims.TenantID)
		if claims.DeptID != "" {
			c.Set("dept_id", claims.DeptID)
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
