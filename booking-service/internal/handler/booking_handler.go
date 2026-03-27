package handler

import (
	"errors"
	"net/http"
	"strconv"

	"diploma/booking-service/internal/client"
	"diploma/booking-service/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateBooking(c *gin.Context) {
	var req struct {
		RequestID       int `json:"request_id"`
		WorkerProfileID int `json:"worker_profile_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}
	if req.RequestID <= 0 || req.WorkerProfileID <= 0 {
		c.JSON(400, gin.H{"error": "request_id and worker_profile_id must be positive"})
		return
	}

	role := c.GetString("role")
	if role != "customer" {
		c.JSON(403, gin.H{"error": "only customers allowed"})
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

	err = h.bookingService.CreateBooking(
		c.Request.Context(),
		req.RequestID,
		req.WorkerProfileID,
		customerProfileID,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrRequestNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrRequestUnavailable),
			errors.Is(err, service.ErrWorkerNotSelectable):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "booking created",
	})
}

func (h *Handler) StartBooking(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}

	role := c.GetString("role")
	if role != "worker" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only workers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	workerProfileID, err := h.userClient.GetWorkerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.bookingService.StartBooking(c.Request.Context(), bookingID, workerProfileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "booking started"})
}

func (h *Handler) CompleteBooking(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}

	role := c.GetString("role")
	if role != "worker" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only workers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	workerProfileID, err := h.userClient.GetWorkerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.bookingService.CompleteBooking(c.Request.Context(), bookingID, workerProfileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "booking completed"})
}
