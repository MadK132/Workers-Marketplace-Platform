package handler

import (
	"errors"
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

	role := c.GetString("role")
	if role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}
	token := c.GetHeader("Authorization")

	customerProfileID, err := h.userClient.GetCustomerProfile(
		c.Request.Context(),
		token,
	)
	if err != nil {
		if errors.Is(err, client.ErrCustomerProfileNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "customer profile not found; create it via POST /api/customer/profile in user-service",
			})
			return
		}
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

func (h *Handler) ListMyRequests(c *gin.Context) {
	role := c.GetString("role")
	if role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, client.ErrCustomerProfileNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "customer profile not found; create it via POST /api/customer/profile in user-service",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	requests, err := h.requestService.ListCustomerRequests(c.Request.Context(), customerProfileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (h *Handler) ListMyBookings(c *gin.Context) {
	role := c.GetString("role")
	token := c.GetHeader("Authorization")

	switch role {
	case "customer":
		customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, client.ErrCustomerProfileNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "customer profile not found; create it via POST /api/customer/profile in user-service",
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		bookings, err := h.bookingService.ListCustomerBookings(c.Request.Context(), customerProfileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, bookings)
		return

	case "worker":
		workerProfileID, err := h.userClient.GetWorkerProfile(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		bookings, err := h.bookingService.ListWorkerBookings(c.Request.Context(), workerProfileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, bookings)
		return

	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers and workers allowed"})
	}
}
