package model

import "time"

type Notification struct {
	ID          int       `json:"notification_id"`
	UserID      int       `json:"user_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	ActionType  string    `json:"action_type,omitempty"`
	ActionRef   string    `json:"action_ref,omitempty"`
	ActionLabel string    `json:"action_label,omitempty"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}
