package realtime

import (
	"encoding/json"
	"sync"

	"diploma/notification-service/internal/model"
)

type Event struct {
	Type         string              `json:"type"`
	UnreadCount  int64               `json:"unread_count,omitempty"`
	Notification *model.Notification `json:"notification,omitempty"`
	Error        string              `json:"error,omitempty"`
}

type Client struct {
	UserID int64
	Send   chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int64]map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*Client]struct{})
	}
	h.clients[client.UserID][client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	userClients := h.clients[client.UserID]
	if userClients == nil {
		return
	}
	if _, ok := userClients[client]; ok {
		delete(userClients, client)
		close(client.Send)
	}
	if len(userClients) == 0 {
		delete(h.clients, client.UserID)
	}
}

func (h *Hub) Broadcast(userID int64, event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients[userID] {
		select {
		case client.Send <- payload:
		default:
			delete(h.clients[userID], client)
			close(client.Send)
		}
	}

	if len(h.clients[userID]) == 0 {
		delete(h.clients, userID)
	}
}
