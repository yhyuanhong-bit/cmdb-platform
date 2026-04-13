package middleware

import (
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// WSAuth returns a Gin middleware that authenticates WebSocket upgrade requests.
// It extracts JWT tokens from:
//  1. Standard Authorization Bearer header
//  2. Sec-WebSocket-Protocol header (format: "access_token.<jwt>")
//  3. Query parameter "token" (legacy fallback, to be removed)
func WSAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// 1. Try Authorization header first
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}

		// 2. Try Sec-WebSocket-Protocol header
		if token == "" {
			for _, proto := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
				proto = strings.TrimSpace(proto)
				if strings.HasPrefix(proto, "access_token.") {
					token = strings.TrimPrefix(proto, "access_token.")
					// Echo the subprotocol back so the browser accepts the connection
					c.Header("Sec-WebSocket-Protocol", proto)
					break
				}
			}
		}

		// 3. Legacy fallback: query parameter (to be removed after migration period)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			response.Err(c, 401, "INVALID_TOKEN", "missing authentication token")
			c.Abort()
			return
		}

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
