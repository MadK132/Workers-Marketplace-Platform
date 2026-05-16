package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"diploma/booking-service/internal/client"
	"diploma/booking-service/internal/filestorage"
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
	hasPaymentMethod, err := h.userClient.HasPaymentMethod(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !hasPaymentMethod {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment method is required"})
		return
	}

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

	evidence := ""
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		file, err := c.FormFile("evidence_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "completion photo is required"})
			return
		}
		url, err := filestorage.SaveUploadedFile(c.Request.Context(), file, filestorage.SaveOptions{
			Prefix:  "booking-evidence",
			MaxSize: 8 * 1024 * 1024,
			AllowedExts: map[string]bool{
				".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
			},
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		evidence = url
	} else {
		var req struct {
			Evidence string `json:"evidence"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		evidence = strings.TrimSpace(req.Evidence)
	}

	err = h.bookingService.CompleteBooking(c.Request.Context(), bookingID, workerProfileID, evidence)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition),
			errors.Is(err, service.ErrEvidenceRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "completion evidence sent; waiting for customer confirmation"})
}

func (h *Handler) RejectBooking(c *gin.Context) {
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

	err = h.bookingService.RejectBooking(c.Request.Context(), bookingID, workerProfileID)
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

	c.JSON(http.StatusOK, gin.H{"message": "booking rejected"})
}

func (h *Handler) ConfirmCompletion(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}

	role := c.GetString("role")
	if role != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := h.bookingService.ConfirmCompletion(c.Request.Context(), bookingID, customerProfileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotCustomer):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrPaymentAmountInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	payment, err := h.paymentClient.CreatePayment(c.Request.Context(), bookingID, amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.bookingService.MarkCompletionPaid(c.Request.Context(), bookingID, customerProfileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "booking completed; payment created",
		"payment_id":     payment.PaymentID,
		"payment_status": payment.Status,
		"payment_url":    payment.URL,
	})
}
