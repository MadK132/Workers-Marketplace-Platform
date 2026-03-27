package router

import (
	"diploma/booking-service/internal/auth"
	"diploma/booking-service/internal/handler"
	"diploma/booking-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	h *handler.Handler,
	tokenManager *auth.TokenManager,
	gatewaySharedSecret string,
) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(tokenManager, gatewaySharedSecret))
	{
		api.POST("/requests", h.CreateRequest)
		api.POST("/bookings", h.CreateBooking)
	}

	return r
}
