package middleware

import (
	"log"
	"runtime/debug"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Recovery returns a middleware that recovers from panics, logs the stack trace,
// and returns a 500 response with code INTERNAL_ERROR.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered: %v\n%s", r, debug.Stack())
				response.InternalError(c, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
