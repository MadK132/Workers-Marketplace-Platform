package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GatewayOnly(sharedSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sharedSecret == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-Gateway-Secret") != sharedSecret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid gateway secret",
			})
			return
		}
		c.Next()
	}
}
