package handler

import (
	"net/http"

	"diploma/booking-service/internal/client"
	"diploma/booking-service/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	requestService *service.RequestService
	bookingService *service.BookingService
	userClient     *client.UserClient
}

func NewHandler(
	requestService *service.RequestService,
	bookingService *service.BookingService,
	userClient *client.UserClient,

) *Handler {
	return &Handler{
		requestService: requestService,
		bookingService: bookingService,
		userClient:     userClient,
	}
}

func (h *Handler) CreateRequest(c *gin.Context) {
	var req struct {
		CategoryID  int    `json:"category_id"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	userID := c.GetInt("user_id")
	token := c.GetHeader("Authorization")

	customerProfileID, err := h.userClient.GetCustomerProfile(
		c.Request.Context(),
		userID,
		token,
	)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err = h.requestService.CreateRequest(
		c.Request.Context(),
		customerProfileID,
		req.CategoryID,
		req.Description,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "request created",
	})
}
