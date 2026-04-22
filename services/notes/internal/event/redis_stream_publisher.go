package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStreamPublisher publishes events through Redis Streams.
//
// It writes messages with XADD so they stay persisted in Redis,
// even if consumers are temporarily offline.
type RedisStreamPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisStreamPublisher creates a Redis Stream publisher.
func NewRedisStreamPublisher(client *redis.Client, logger *zap.Logger) *RedisStreamPublisher {
	return &RedisStreamPublisher{client: client, logger: logger}
}

// Publish writes an event to a Redis Stream.
//
// The stream key matches the topic name, for example "snippet.saved".
// Messages contain a "data" field whose value is the JSON-serialized payload.
func (p *RedisStreamPublisher) Publish(ctx context.Context, topic Topic, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: failed to serialize payload: %w", err)
	}

	_, err = p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: string(topic),
		Values: map[string]any{
			"data": string(data),
		},
		// MaxLen + Approx keeps the most recent 10,000 messages and trims old ones automatically.
		MaxLen: 10000,
		Approx: true,
	}).Result()
	if err != nil {
		p.logger.Error("event: XADD failed",
			zap.String("topic", string(topic)),
			zap.Error(err),
		)
		return fmt.Errorf("event: XADD failed: %w", err)
	}

	p.logger.Debug("event: published",
		zap.String("topic", string(topic)),
		zap.ByteString("data", data),
	)
	return nil
}

// Close releases resources. No extra cleanup is currently required.
func (p *RedisStreamPublisher) Close() error {
	return nil
}
