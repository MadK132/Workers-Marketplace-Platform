package router

import (
	"diploma/notification-service/internal/config"
	"diploma/notification-service/internal/handler"
	"diploma/notification-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(h *handler.Handler, cfg config.Config) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Health)

	internal := r.Group("/internal")
	internal.Use(middleware.GatewayOnly(cfg.Gateway.SharedSecret))
	{
		internal.POST("/notifications", h.CreateInternal)
	}

	api := r.Group("/api")
	api.Use(middleware.GatewayOnly(cfg.Gateway.SharedSecret))
	{
		api.GET("/notifications", h.ListMine)
		api.PATCH("/notifications/:notification_id/read", h.MarkRead)
		api.PATCH("/notifications/read-all", h.MarkAllRead)
	}

	return r
}
