package model

import "time"

type Chat struct {
	ChatID         int64     `json:"chat_id"`
	BookingID      int64     `json:"booking_id"`
	CustomerUserID int64     `json:"customer_user_id"`
	WorkerUserID   int64     `json:"worker_user_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	UnreadCount    int64     `json:"unread_count,omitempty"`
}

type Message struct {
	MessageID    int64      `json:"message_id"`
	ChatID       int64      `json:"chat_id"`
	SenderUserID int64      `json:"sender_user_id"`
	Content      string     `json:"content"`
	CreatedAt    time.Time  `json:"created_at"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
}
