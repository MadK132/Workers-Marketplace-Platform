package notifications

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	client  *redis.Client
	channel string
}

func NewRedisPublisher(client *redis.Client, channel string) *RedisPublisher {
	return &RedisPublisher{
		client:  client,
		channel: channel,
	}
}

func (p *RedisPublisher) Publish(ctx context.Context, event Event) error {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.client.Publish(ctx, p.channel, payload).Err()
}
