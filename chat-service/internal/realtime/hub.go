package realtime

import (
	"encoding/json"
	"sync"

	"diploma/chat-service/internal/model"
)

type Event struct {
	Type    string         `json:"type"`
	ChatID  int64          `json:"chat_id"`
	Message *model.Message `json:"message,omitempty"`
	Error   string         `json:"error,omitempty"`
	NodeID  string         `json:"node_id,omitempty"`
}

type Client struct {
	ChatID int64
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

	if h.clients[client.ChatID] == nil {
		h.clients[client.ChatID] = make(map[*Client]struct{})
	}
	h.clients[client.ChatID][client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	chatClients := h.clients[client.ChatID]
	if chatClients == nil {
		return
	}

	if _, ok := chatClients[client]; ok {
		delete(chatClients, client)
		close(client.Send)
	}

	if len(chatClients) == 0 {
		delete(h.clients, client.ChatID)
	}
}

func (h *Hub) Broadcast(chatID int64, event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients[chatID] {
		select {
		case client.Send <- payload:
		default:
			delete(h.clients[chatID], client)
			close(client.Send)
		}
	}

	if len(h.clients[chatID]) == 0 {
		delete(h.clients, chatID)
	}
}
