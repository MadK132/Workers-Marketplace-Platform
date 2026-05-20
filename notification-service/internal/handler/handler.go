package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"diploma/notification-service/internal/model"
	"diploma/notification-service/internal/service"

	"github.com/gin-gonic/gin"
)

type NotificationService interface {
	Create(ctx context.Context, userID int, notificationType string, title string, message string, actionType string, actionRef string, actionLabel string) (model.Notification, error)
	ListByUser(ctx context.Context, userID int, limit int, onlyUnread bool) ([]model.Notification, error)
	MarkRead(ctx context.Context, notificationID int, userID int) (model.Notification, error)
	MarkAllRead(ctx context.Context, userID int) (int64, error)
}

type Handler struct {
	notifications NotificationService
}

func New(notifications NotificationService) *Handler {
	return &Handler{notifications: notifications}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) CreateInternal(c *gin.Context) {
	var req struct {
		UserID      int    `json:"user_id"`
		Type        string `json:"type"`
		Title       string `json:"title"`
		Message     string `json:"message"`
		ActionType  string `json:"action_type"`
		ActionRef   string `json:"action_ref"`
		ActionLabel string `json:"action_label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	notification, err := h.notifications.Create(
		c.Request.Context(),
		req.UserID,
		req.Type,
		req.Title,
		req.Message,
		req.ActionType,
		req.ActionRef,
		req.ActionLabel,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, notification)
}

func (h *Handler) ListMine(c *gin.Context) {
	userID := c.GetInt("user_id")
	limit := parseIntQuery(c, "limit", 50)
	onlyUnread := c.Query("unread") == "true" || c.Query("unread") == "1"

	notifications, err := h.notifications.ListByUser(
		c.Request.Context(),
		userID,
		limit,
		onlyUnread,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, notifications)
}

func (h *Handler) MarkRead(c *gin.Context) {
	notificationID, ok := parseIDParam(c, "notification_id")
	if !ok {
		return
	}

	notification, err := h.notifications.MarkRead(
		c.Request.Context(),
		notificationID,
		c.GetInt("user_id"),
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, notification)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	count, err := h.notifications.MarkAllRead(c.Request.Context(), c.GetInt("user_id"))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"read_notifications": count})
}

func parseIDParam(c *gin.Context, name string) (int, bool) {
	raw := c.Param(name)
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return 0, false
	}
	return id, true
}

func parseIntQuery(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNotificationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
