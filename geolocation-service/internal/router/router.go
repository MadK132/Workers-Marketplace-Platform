package router

import (
	"diploma/geolocation-service/internal/config"
	"diploma/geolocation-service/internal/handler"
	"diploma/geolocation-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Setup(h *handler.Handler, cfg config.Config) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Health)

	api := r.Group("/api")
	api.Use(middleware.GatewayOnly(cfg.Gateway.SharedSecret))
	{
		api.GET("/geo/workers/nearby", h.FindNearbyWorkers)
	}

	return r
}
