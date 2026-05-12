package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"diploma/chat-service/internal/auth"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(tokens *auth.TokenManager, gatewaySharedSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if gatewaySharedSecret != "" &&
			c.GetHeader("X-Gateway-Secret") == gatewaySharedSecret {
			userIDStr := c.GetHeader("X-User-ID")
			role := c.GetHeader("X-Role")
			userID, err := strconv.Atoi(userIDStr)
			if err == nil && userID > 0 && role != "" {
				c.Set("user_id", userID)
				c.Set("role", role)
				c.Next()
				return
			}
		}

		token := bearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing Authorization header",
			})
			return
		}

		claims, err := tokens.Parse(token)
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

func bearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return c.Query("access_token")
}
