package middleware

import (
	"net/http"
	"strconv"

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
		setTrustedIdentity(c)
		c.Next()
	}
}

func setTrustedIdentity(c *gin.Context) {
	userID, err := strconv.Atoi(c.GetHeader("X-User-ID"))
	if err == nil && userID > 0 {
		c.Set("user_id", userID)
	}
	if role := c.GetHeader("X-Role"); role != "" {
		c.Set("role", role)
	}
}
