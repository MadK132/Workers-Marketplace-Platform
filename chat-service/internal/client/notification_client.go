package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type NotificationClient struct {
	baseURL       string
	gatewaySecret string
	httpClient    *http.Client
}

func NewNotificationClient(baseURL string, gatewaySecret string) *NotificationClient {
	return &NotificationClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		gatewaySecret: gatewaySecret,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (c *NotificationClient) Create(ctx context.Context, userID int64, notificationType string, title string, message string) {
	c.CreateAction(ctx, userID, notificationType, title, message, "", "", "")
}

func (c *NotificationClient) CreateAction(ctx context.Context, userID int64, notificationType string, title string, message string, actionType string, actionRef string, actionLabel string) {
	if c == nil || c.baseURL == "" || userID <= 0 {
		return
	}

	payload, err := json.Marshal(map[string]any{
		"user_id":      userID,
		"type":         notificationType,
		"title":        title,
		"message":      message,
		"action_type":  actionType,
		"action_ref":   actionRef,
		"action_label": actionLabel,
	})
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/notifications", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if c.gatewaySecret != "" {
		req.Header.Set("X-Gateway-Secret", c.gatewaySecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
