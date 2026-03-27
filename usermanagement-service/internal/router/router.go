package router

import (
	"github.com/gin-gonic/gin"

	"diploma/usermanagement-service/internal/auth"
	"diploma/usermanagement-service/internal/handler"
	"diploma/usermanagement-service/internal/middleware"
)

func SetupRouter(h *handler.AuthHandler, tokens *auth.TokenManager) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Health)

	auth := r.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.GET("/verify", h.Verify)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
		auth.POST("/resend-verification", h.ResendVerification)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
	}

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(tokens))
	{
		api.GET("/test", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			role, _ := c.Get("role")

			c.JSON(200, gin.H{
				"user_id": userID,
				"role":    role,
			})
		})
	}

	return r
}
