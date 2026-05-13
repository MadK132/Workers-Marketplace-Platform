package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"diploma/internal/notifications"
	"diploma/notification-service/internal/realtime"
	"diploma/notification-service/internal/repository"
	notificationservice "diploma/notification-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	service  *notificationservice.NotificationService
	hub      *realtime.Hub
	upgrader websocket.Upgrader
}

func New(service *notificationservice.NotificationService, hub *realtime.Hub) *Handler {
	return &Handler{
		service: service,
		hub:     hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) CreateInternal(c *gin.Context) {
	var event notifications.Event
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	item, err := h.service.CreateFromEvent(c.Request.Context(), event)
	if err != nil {
		respondError(c, err)
		return
	}

	unreadCount, err := h.service.CountUnread(c.Request.Context(), item.UserID)
	if err != nil {
		unreadCount = 0
	}
	h.hub.Broadcast(item.UserID, realtime.Event{
		Type:         "notification.created",
		UnreadCount:  unreadCount,
		Notification: &item,
	})

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) List(c *gin.Context) {
	limit := parseIntQuery(c, "limit", 50)
	unreadOnly := c.Query("unread_only") == "true"

	items, err := h.service.List(c.Request.Context(), currentUserID(c), limit, unreadOnly)
	if err != nil {
		respondError(c, err)
		return
	}

	unreadCount, err := h.service.CountUnread(c.Request.Context(), currentUserID(c))
	if err != nil {
		unreadCount = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": items,
		"unread_count":  unreadCount,
	})
}

func (h *Handler) CountUnread(c *gin.Context) {
	count, err := h.service.CountUnread(c.Request.Context(), currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

func (h *Handler) MarkRead(c *gin.Context) {
	notificationID, ok := parseIDParam(c, "notification_id")
	if !ok {
		return
	}

	item, err := h.service.MarkRead(c.Request.Context(), notificationID, currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	unreadCount, err := h.service.CountUnread(c.Request.Context(), currentUserID(c))
	if err != nil {
		unreadCount = 0
	}
	h.hub.Broadcast(currentUserID(c), realtime.Event{
		Type:         "notification.read",
		UnreadCount:  unreadCount,
		Notification: &item,
	})

	c.JSON(http.StatusOK, item)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	count, err := h.service.MarkAllRead(c.Request.Context(), currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	h.hub.Broadcast(currentUserID(c), realtime.Event{
		Type:        "notifications.read_all",
		UnreadCount: 0,
	})

	c.JSON(http.StatusOK, gin.H{"read_notifications": count})
}

func (h *Handler) WebSocket(c *gin.Context) {
	userID := currentUserID(c)
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &realtime.Client{
		UserID: userID,
		Send:   make(chan []byte, 32),
	}
	h.hub.Register(client)

	unreadCount, err := h.service.CountUnread(c.Request.Context(), userID)
	if err == nil {
		sendClientEvent(client, realtime.Event{
			Type:        "notifications.ready",
			UnreadCount: unreadCount,
		})
	}

	go h.writePump(conn, client)
	h.readPump(conn, client)
}

func (h *Handler) readPump(conn *websocket.Conn, client *realtime.Client) {
	defer func() {
		h.hub.Unregister(client)
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Handler) writePump(conn *websocket.Conn, client *realtime.Client) {
	for payload := range client.Send {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
	_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
}

func sendClientEvent(client *realtime.Client, event realtime.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	select {
	case client.Send <- payload:
	default:
	}
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
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

func currentUserID(c *gin.Context) int64 {
	return int64(c.GetInt("user_id"))
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, notificationservice.ErrInvalidNotification):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrNotificationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
