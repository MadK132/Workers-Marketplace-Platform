package router

import (
	"github.com/gin-gonic/gin"

	"diploma/usermanagement-service/internal/auth"
	"diploma/usermanagement-service/internal/handler"
	"diploma/usermanagement-service/internal/middleware"
)

func SetupRouter(
	h *handler.AuthHandler,
	tokens *auth.TokenManager,
	gatewaySharedSecret string,
) *gin.Engine {
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
		auth.GET("/reset", h.ResetPasswordPage)
		auth.POST("/reset-password", h.ResetPassword)
		auth.POST("/select-role", middleware.AuthMiddleware(tokens, gatewaySharedSecret), h.SelectRole)

	}

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(tokens, gatewaySharedSecret))
	{
		api.POST("/customer/profile", h.CreateCustomerProfile)
		api.GET("/customer/profile", h.GetCustomerProfile)
		api.GET("/worker/profile", h.GetWorkerProfile)
		api.POST("/worker/profile", h.CreateWorkerProfile)
		api.POST("/worker/skills", h.AddWorkerSkill)
		api.POST("/admin/verify-skill", h.VerifyWorkerSkill)
		api.PATCH("/worker/availability", h.SetAvailability)
		api.GET("/categories", h.GetCategories)
		api.GET("/workers", h.FindWorkers)
		api.GET("/admin/overview", h.AdminOverview)
		api.GET("/admin/users", h.AdminUsers)
		api.POST("/admin/admins", h.AdminCreateAdmin)
		api.POST("/admin/managers", h.AdminCreateManager)
		api.PATCH("/admin/users/:id/activate", h.AdminActivateUser)
		api.DELETE("/admin/users/:id", h.AdminDeleteUser)
		api.GET("/internal/customer-profile", h.GetCustomerProfile)
		api.GET("/internal/worker-profile", h.GetWorkerProfile)
	}

	return r
}
