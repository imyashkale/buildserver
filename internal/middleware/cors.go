package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/imyashkale/buildserver/internal/logger"
)

// CORS returns a middleware that handles CORS
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			logger.WithFields(map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
				"origin": c.Request.Header.Get("Origin"),
			}).Debug("CORS preflight request handled")
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
