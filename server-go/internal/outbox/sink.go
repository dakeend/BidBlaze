package outbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Sink interface {
	Publish(ctx context.Context, event Event) error
}

type RedisSink struct {
	client        *redis.Client
	channelPrefix string
}

func NewRedisSink(client *redis.Client, channelPrefix string) *RedisSink {
	if channelPrefix == "" {
		channelPrefix = "auction:events:"
	}
	return &RedisSink{
		client:        client,
		channelPrefix: channelPrefix,
	}
}

func (s *RedisSink) Publish(ctx context.Context, event Event) error {
	channel := fmt.Sprintf("%s%d", s.channelPrefix, event.AggregateID)
	subscribers, err := s.client.Publish(ctx, channel, []byte(event.Payload)).Result()
	if err != nil {
		return err
	}
	if subscribers == 0 {
		return ErrNoGatewaySubscriber
	}
	return nil
}

type ChannelSink struct {
	events chan Event
}

func NewChannelSink(buffer int) *ChannelSink {
	if buffer <= 0 {
		buffer = 256
	}
	return &ChannelSink{events: make(chan Event, buffer)}
}

func (s *ChannelSink) Publish(ctx context.Context, event Event) error {
	select {
	case s.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ChannelSink) Events() <-chan Event {
	return s.events
}

var ErrNoGatewaySubscriber = errors.New("no WebSocket gateway subscribed to Redis channel")
