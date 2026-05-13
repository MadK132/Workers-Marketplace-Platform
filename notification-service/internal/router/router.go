package router

import (
	"diploma/notification-service/internal/auth"
	"diploma/notification-service/internal/handler"
	"diploma/notification-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(
	h *handler.Handler,
	tokenManager *auth.TokenManager,
	gatewaySharedSecret string,
) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Health)

	internal := r.Group("/internal")
	internal.Use(middleware.GatewayOnly(gatewaySharedSecret))
	{
		internal.POST("/notifications", h.CreateInternal)
	}

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(tokenManager, gatewaySharedSecret))
	{
		api.GET("/notifications", h.List)
		api.GET("/notifications/unread-count", h.CountUnread)
		api.PATCH("/notifications/read-all", h.MarkAllRead)
		api.PATCH("/notifications/:notification_id/read", h.MarkRead)
		api.GET("/notifications/ws", h.WebSocket)
	}

	return r
}
