package realtime

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"diploma/internal/notifications"
	notificationservice "diploma/notification-service/internal/service"

	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	client  *redis.Client
	channel string
	service *notificationservice.NotificationService
	hub     *Hub
}

func NewRedisConsumer(
	client *redis.Client,
	channel string,
	service *notificationservice.NotificationService,
	hub *Hub,
) *RedisConsumer {
	return &RedisConsumer{
		client:  client,
		channel: channel,
		service: service,
		hub:     hub,
	}
}

func (c *RedisConsumer) Run(ctx context.Context) {
	sub := c.client.Subscribe(ctx, c.channel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event notifications.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("notification event decode error: %v", err)
				continue
			}

			saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			item, err := c.service.CreateFromEvent(saveCtx, event)
			if err != nil {
				cancel()
				log.Printf("notification event save error: %v", err)
				continue
			}
			unreadCount, err := c.service.CountUnread(saveCtx, item.UserID)
			cancel()
			if err != nil {
				log.Printf("notification unread count error: %v", err)
			}

			c.hub.Broadcast(item.UserID, Event{
				Type:         "notification.created",
				UnreadCount:  unreadCount,
				Notification: &item,
			})
		}
	}
}
