package model

import (
	"encoding/json"
	"time"
)

type Notification struct {
	ID            int64           `json:"notification_id"`
	UserID        int64           `json:"user_id"`
	SourceService string          `json:"source_service"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Body          string          `json:"body"`
	Data          json.RawMessage `json:"data"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	ReadAt        *time.Time      `json:"read_at,omitempty"`
}
