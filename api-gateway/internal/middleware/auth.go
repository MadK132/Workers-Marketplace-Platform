package middleware

import (
	"net/http"
	"strings"

	"diploma/api-gateway/internal/auth"

	"github.com/gin-gonic/gin"
)

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing Authorization header",
			})
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid Authorization format",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ParseJWT(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
