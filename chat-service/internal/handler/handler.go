package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"diploma/chat-service/internal/client"
	"diploma/chat-service/internal/filestorage"
	"diploma/chat-service/internal/model"
	"diploma/chat-service/internal/realtime"
	"diploma/chat-service/internal/repository"
	chatservice "diploma/chat-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	service       *chatservice.ChatService
	hub           *realtime.Hub
	publisher     realtime.Publisher
	notifications *client.NotificationClient
	upgrader      websocket.Upgrader
}

func NewHandler(
	service *chatservice.ChatService,
	hub *realtime.Hub,
	publisher realtime.Publisher,
	notifications *client.NotificationClient,
) *Handler {
	return &Handler{
		service:       service,
		hub:           hub,
		publisher:     publisher,
		notifications: notifications,
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

func (h *Handler) CreateChat(c *gin.Context) {
	var req struct {
		BookingID      int64 `json:"booking_id"`
		CustomerUserID int64 `json:"customer_user_id"`
		WorkerUserID   int64 `json:"worker_user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	chat, err := h.service.CreateChat(
		c.Request.Context(),
		currentUserID(c),
		c.GetString("role"),
		req.BookingID,
		req.CustomerUserID,
		req.WorkerUserID,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, chat)
}

func (h *Handler) ListChats(c *gin.Context) {
	chats, err := h.service.ListChats(c.Request.Context(), currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, chats)
}

func (h *Handler) ListMessages(c *gin.Context) {
	chatID, ok := parseIDParam(c, "chat_id")
	if !ok {
		return
	}

	limit := parseIntQuery(c, "limit", 50)
	beforeID := int64(parseIntQuery(c, "before_id", 0))

	messages, err := h.service.ListMessages(
		c.Request.Context(),
		chatID,
		currentUserID(c),
		limit,
		beforeID,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *Handler) SendMessage(c *gin.Context) {
	chatID, ok := parseIDParam(c, "chat_id")
	if !ok {
		return
	}

	content := ""
	attachmentURL := ""
	attachmentName := ""
	attachmentType := ""

	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		content = strings.TrimSpace(c.PostForm("content"))
		file, err := c.FormFile("attachment")
		if err == nil {
			url, err := filestorage.SaveUploadedFile(c.Request.Context(), file, filestorage.SaveOptions{
				Prefix:  "chat-attachments",
				MaxSize: 12 * 1024 * 1024,
				AllowedExts: map[string]bool{
					".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
					".mp4": true, ".mov": true, ".webm": true,
					".pdf": true, ".doc": true, ".docx": true, ".txt": true,
				},
			})
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			attachmentURL = url
			attachmentName = file.Filename
			attachmentType = file.Header.Get("Content-Type")
		}
	} else {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}
		content = req.Content
	}

	msg, err := h.service.SendMessageWithAttachment(
		c.Request.Context(),
		chatID,
		currentUserID(c),
		content,
		attachmentURL,
		attachmentName,
		attachmentType,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	h.publishMessage(c.Request.Context(), msg.ChatID, msg)
	h.notifyChatRecipient(c.Request.Context(), chatID, currentUserID(c), msg)
	c.JSON(http.StatusCreated, msg)
}

func (h *Handler) MarkRead(c *gin.Context) {
	chatID, ok := parseIDParam(c, "chat_id")
	if !ok {
		return
	}

	count, err := h.service.MarkRead(c.Request.Context(), chatID, currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"read_messages": count})
}

func (h *Handler) ChatWebSocket(c *gin.Context) {
	chatID, ok := parseIDParam(c, "chat_id")
	if !ok {
		return
	}

	userID := currentUserID(c)
	if _, err := h.service.GetChat(c.Request.Context(), chatID, userID); err != nil {
		respondError(c, err)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(4096)

	client := &realtime.Client{
		ChatID: chatID,
		UserID: userID,
		Send:   make(chan []byte, 32),
	}
	h.hub.Register(client)

	go h.writePump(conn, client)
	h.readPump(conn, client)
}

func (h *Handler) readPump(conn *websocket.Conn, client *realtime.Client) {
	defer func() {
		h.hub.Unregister(client)
		_ = conn.Close()
	}()

	for {
		var incoming struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if err := conn.ReadJSON(&incoming); err != nil {
			return
		}

		switch incoming.Type {
		case "message":
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msg, err := h.service.SendMessage(ctx, client.ChatID, client.UserID, incoming.Content)
			cancel()
			if err != nil {
				sendClientEvent(client, realtime.Event{
					Type:   "error",
					ChatID: client.ChatID,
					Error:  err.Error(),
				})
				continue
			}
			h.publishMessage(context.Background(), msg.ChatID, msg)
			h.notifyChatRecipient(context.Background(), client.ChatID, client.UserID, msg)
		default:
			sendClientEvent(client, realtime.Event{
				Type:   "error",
				ChatID: client.ChatID,
				Error:  "unsupported websocket message type",
			})
		}
	}
}

func (h *Handler) notifyChatRecipient(ctx context.Context, chatID int64, senderUserID int64, msg model.Message) {
	if h.notifications == nil {
		return
	}
	chat, err := h.service.GetChatForUser(ctx, chatID, senderUserID)
	if err != nil {
		log.Printf("chat notification skipped for chat %d: %v", chatID, err)
		return
	}
	recipientID := chat.CustomerUserID
	if recipientID == senderUserID {
		recipientID = chat.WorkerUserID
	}
	text := strings.TrimSpace(msg.Content)
	if msg.AttachmentURL != "" {
		text = "New file in chat"
	}
	if text == "" {
		text = "New message"
	}
	h.notifications.CreateAction(ctx, recipientID, "chat_message", "New chat message", text, "chat", strconv.FormatInt(chatID, 10), "Open chat")
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

func (h *Handler) publishMessage(ctx context.Context, chatID int64, msg model.Message) {
	event := realtime.Event{
		Type:    "message.created",
		ChatID:  chatID,
		Message: &msg,
	}

	h.hub.Broadcast(chatID, event)
	if err := h.publisher.Publish(ctx, event); err != nil {
		log.Printf("chat redis publish error: %v", err)
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
	case errors.Is(err, chatservice.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, chatservice.ErrMessageTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, chatservice.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrChatNotFound), errors.Is(err, repository.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
