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

	bookingID, err := h.bookingService.CreateBooking(
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

	h.notifyWorkerChatAction(c.Request.Context(), bookingID, "booking_created", "New request", "A customer opened a chat and is waiting for your price.")

	c.JSON(http.StatusOK, gin.H{
		"message":    "booking created",
		"booking_id": bookingID,
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
	h.notifyCustomerMapAction(c.Request.Context(), bookingID, "booking_started", "Worker is on the way", "The worker has started moving to your address.")
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

	h.notifyCustomer(c.Request.Context(), bookingID, "completion_evidence", "Completion evidence sent", "The worker sent completion evidence. Please review and confirm if everything is done.")

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

	h.notifyCustomer(c.Request.Context(), bookingID, "booking_rejected", "Booking rejected", "The worker declined the booking. You can choose another worker.")

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

	h.notifyWorker(c.Request.Context(), bookingID, "booking_completed", "Booking completed", "The customer confirmed completion and payment was created.")

	c.JSON(http.StatusOK, gin.H{
		"message":        "booking completed; payment created",
		"payment_id":     payment.PaymentID,
		"payment_status": payment.Status,
		"payment_url":    payment.URL,
	})
}

func (h *Handler) SetBookingPrice(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}

	var req struct {
		FinalPrice float64 `json:"final_price"`
		Amount     float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	amount := req.FinalPrice
	if amount == 0 {
		amount = req.Amount
	}

	role := c.GetString("role")
	token := c.GetHeader("Authorization")
	switch role {
	case "customer":
		customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err = h.bookingService.SetFinalPriceForCustomer(c.Request.Context(), bookingID, customerProfileID, amount)
	case "worker":
		workerProfileID, err := h.userClient.GetWorkerProfile(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		err = h.bookingService.SetFinalPriceForWorker(c.Request.Context(), bookingID, workerProfileID, amount)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers and workers allowed"})
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotCustomer),
			errors.Is(err, service.ErrBookingNotOwned):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition),
			errors.Is(err, service.ErrPriceInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	if role == "worker" {
		h.notifyCustomer(c.Request.Context(), bookingID, "price_set", "Price offered", "The worker sent a price. Open chat to accept or reject it.")
	} else if role == "customer" {
		h.notifyWorker(c.Request.Context(), bookingID, "price_updated", "Price updated", "The customer updated the booking price.")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "booking price updated",
		"final_price": amount,
	})
}

func (h *Handler) AcceptBookingPrice(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}
	if c.GetString("role") != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.bookingService.AcceptBookingPrice(c.Request.Context(), bookingID, customerProfileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotCustomer):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition),
			errors.Is(err, service.ErrPaymentAmountInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	h.notifyWorker(c.Request.Context(), bookingID, "price_accepted", "Price accepted", "The customer accepted your price. You can start the booking.")

	c.JSON(http.StatusOK, gin.H{"message": "booking price accepted"})
}

func (h *Handler) RejectBookingPrice(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}
	if c.GetString("role") != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}

	token := c.GetHeader("Authorization")
	customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.bookingService.RejectBookingPrice(c.Request.Context(), bookingID, customerProfileID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotCustomer):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	h.notifyWorker(c.Request.Context(), bookingID, "price_rejected", "Price rejected", "The customer rejected your price. You can send a new price in the same chat.")

	c.JSON(http.StatusOK, gin.H{"message": "booking price rejected"})
}

func (h *Handler) CreateReview(c *gin.Context) {
	bookingID, err := strconv.Atoi(c.Param("booking_id"))
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking_id"})
		return
	}

	if c.GetString("role") != "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only customers allowed"})
		return
	}

	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	photoURL := ""
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		rating, err := strconv.Atoi(c.PostForm("rating"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rating"})
			return
		}
		req.Rating = rating
		req.Comment = c.PostForm("comment")
		file, err := c.FormFile("review_photo")
		if err == nil && file != nil {
			url, err := filestorage.SaveUploadedFile(c.Request.Context(), file, filestorage.SaveOptions{
				Prefix:  "review-photos",
				MaxSize: 8 * 1024 * 1024,
				AllowedExts: map[string]bool{
					".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
				},
			})
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			photoURL = url
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
	}

	token := c.GetHeader("Authorization")
	customerProfileID, err := h.userClient.GetCustomerProfile(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewID, err := h.bookingService.CreateReview(
		c.Request.Context(),
		bookingID,
		customerProfileID,
		req.Rating,
		strings.TrimSpace(req.Comment),
		photoURL,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrBookingNotCustomer):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrInvalidTransition),
			errors.Is(err, service.ErrReviewInvalidRating):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	h.notifyWorker(c.Request.Context(), bookingID, "review_created", "New review", "The customer left a review for this booking.")

	c.JSON(http.StatusOK, gin.H{
		"message":   "review saved",
		"review_id": reviewID,
	})
}

func (h *Handler) ListWorkerReviews(c *gin.Context) {
	workerProfileID, err := strconv.Atoi(c.Param("worker_profile_id"))
	if err != nil || workerProfileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker_profile_id"})
		return
	}

	reviews, err := h.bookingService.ListWorkerReviews(c.Request.Context(), workerProfileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, reviews)
}
