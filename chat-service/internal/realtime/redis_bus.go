package realtime

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, Event) error {
	return nil
}

type RedisBus struct {
	client  *redis.Client
	channel string
	nodeID  string
}

func NewRedisBus(client *redis.Client, channel string, nodeID string) *RedisBus {
	return &RedisBus{
		client:  client,
		channel: channel,
		nodeID:  nodeID,
	}
}

func (b *RedisBus) Publish(ctx context.Context, event Event) error {
	event.NodeID = b.nodeID

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return b.client.Publish(ctx, b.channel, payload).Err()
}

func (b *RedisBus) Run(ctx context.Context, hub *Hub) {
	sub := b.client.Subscribe(ctx, b.channel)
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

			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("chat redis event decode error: %v", err)
				continue
			}
			if event.NodeID == b.nodeID {
				continue
			}
			event.NodeID = ""
			hub.Broadcast(event.ChatID, event)
		}
	}
}
