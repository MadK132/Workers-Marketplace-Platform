package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GatewayOnly(sharedSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sharedSecret != "" && c.GetHeader("X-Gateway-Secret") != sharedSecret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid gateway secret",
			})
			return
		}

		userID, _ := strconv.Atoi(c.GetHeader("X-User-ID"))
		if userID > 0 {
			c.Set("user_id", userID)
		}
		if role := c.GetHeader("X-Role"); role != "" {
			c.Set("role", role)
		}

		c.Next()
	}
}
