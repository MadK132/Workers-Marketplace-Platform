package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidEvent = errors.New("invalid notification event")

type Event struct {
	ID              string         `json:"id,omitempty"`
	SourceService   string         `json:"source_service"`
	Type            string         `json:"type"`
	RecipientUserID int64          `json:"recipient_user_id"`
	Title           string         `json:"title"`
	Body            string         `json:"body"`
	Data            map[string]any `json:"data,omitempty"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, Event) error {
	return nil
}

func (e Event) Validate() error {
	if e.RecipientUserID <= 0 ||
		strings.TrimSpace(e.SourceService) == "" ||
		strings.TrimSpace(e.Type) == "" ||
		strings.TrimSpace(e.Title) == "" ||
		strings.TrimSpace(e.Body) == "" {
		return ErrInvalidEvent
	}
	return nil
}

func (e Event) Normalize() Event {
	e.SourceService = strings.TrimSpace(e.SourceService)
	e.Type = strings.TrimSpace(e.Type)
	e.Title = strings.TrimSpace(e.Title)
	e.Body = strings.TrimSpace(e.Body)
	if e.Data == nil {
		e.Data = map[string]any{}
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	return e
}

func (e Event) DataJSON() []byte {
	if e.Data == nil {
		return []byte(`{}`)
	}
	payload, err := json.Marshal(e.Data)
	if err != nil {
		return []byte(`{}`)
	}
	return payload
}
