package middleware

import "github.com/gin-gonic/gin"

// TenantContext is a middleware that ensures tenant_id is present in the gin
// context. Currently this is a no-op passthrough because Auth already sets
// tenant_id. It exists as a hook for future tenant validation logic.
func TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
