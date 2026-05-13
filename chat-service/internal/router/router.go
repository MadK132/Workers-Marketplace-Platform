package router

import (
	"diploma/chat-service/internal/auth"
	"diploma/chat-service/internal/handler"
	"diploma/chat-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	h *handler.Handler,
	tokenManager *auth.TokenManager,
	gatewaySharedSecret string,
) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", h.Health)

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(tokenManager, gatewaySharedSecret))
	{
		api.POST("/chats", h.CreateChat)
		api.GET("/chats", h.ListChats)
		api.GET("/chats/:chat_id/messages", h.ListMessages)
		api.POST("/chats/:chat_id/messages", h.SendMessage)
		api.PATCH("/chats/:chat_id/read", h.MarkRead)
		api.GET("/chats/:chat_id/ws", h.ChatWebSocket)
	}

	return r
}
