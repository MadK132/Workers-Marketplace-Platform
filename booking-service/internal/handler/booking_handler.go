package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateBooking(c *gin.Context) {
	var req struct {
		RequestID int `json:"request_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	role := c.GetString("role")
	if role != "worker" {
		c.JSON(403, gin.H{"error": "only workers allowed"})
		return
	}

	userID := c.GetInt("user_id")
	token := c.GetHeader("Authorization")

	workerProfileID, err := h.userClient.GetWorkerProfile(
		c.Request.Context(),
		userID,
		token,
	)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err = h.bookingService.CreateBooking(
		c.Request.Context(),
		req.RequestID,
		workerProfileID,
	)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "booking created",
	})
}
