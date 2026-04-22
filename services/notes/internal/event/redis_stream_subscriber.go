package event

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStreamSubscriber subscribes to events through a Redis Stream consumer group.
//
// Flow:
//  1. Create the consumer group on startup if needed (XGROUP CREATE ... MKSTREAM)
//  2. Poll for new messages with XREADGROUP
//  3. ACK with XACK when the handler succeeds
//  4. Leave the message pending when the handler fails so it can be retried after restart
type RedisStreamSubscriber struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisStreamSubscriber creates a Redis Stream subscriber.
func NewRedisStreamSubscriber(client *redis.Client, logger *zap.Logger) *RedisStreamSubscriber {
	return &RedisStreamSubscriber{client: client, logger: logger}
}

// Subscribe consumes a topic through a consumer group and blocks until ctx is canceled.
func (s *RedisStreamSubscriber) Subscribe(ctx context.Context, topic Topic, group, consumer string, handler Handler) error {
	stream := string(topic)

	// 1. Create the consumer group idempotently.
	if err := s.ensureGroup(ctx, stream, group); err != nil {
		return err
	}

	s.logger.Info("event: starting subscription",
		zap.String("topic", stream),
		zap.String("group", group),
		zap.String("consumer", consumer),
	)

	// 2. Drain pending messages first, then process new ones.
	if err := s.processPending(ctx, stream, group, consumer, handler); err != nil {
		s.logger.Warn("event: failed to process pending messages", zap.Error(err))
	}

	// 3. Continuously read new messages.
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("event: subscription stopped", zap.String("topic", stream))
			return ctx.Err()
		default:
		}

		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"}, // ">" means read new messages only.
			Count:    10,                    // Read at most 10 messages per batch.
			Block:    3 * time.Second,       // Block for up to 3 seconds.
		}).Result()

		if err != nil {
			if err == redis.Nil || err == context.Canceled || err == context.DeadlineExceeded {
				continue // No message before timeout, or ctx was canceled.
			}
			s.logger.Error("event: XREADGROUP failed", zap.Error(err))
			time.Sleep(time.Second) // Prevent error storms.
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				s.handleMessage(ctx, stream.Stream, group, msg, handler)
			}
		}
	}
}

// processPending replays pending messages left unacked by a previous worker run.
func (s *RedisStreamSubscriber) processPending(ctx context.Context, stream, group, consumer string, handler Handler) error {
	for {
		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, "0"}, // "0" means read pending messages.
			Count:    10,
		}).Result()
		if err != nil {
			return fmt.Errorf("event: failed to read pending messages: %w", err)
		}

		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			return nil // No pending messages left.
		}

		for _, msg := range streams[0].Messages {
			s.handleMessage(ctx, stream, group, msg, handler)
		}
	}
}

// handleMessage processes one message and ACKs it on success.
func (s *RedisStreamSubscriber) handleMessage(ctx context.Context, stream, group string, msg redis.XMessage, handler Handler) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		s.logger.Warn("event: message missing data field, auto-ACK and skip",
			zap.String("msg_id", msg.ID),
		)
		s.client.XAck(ctx, stream, group, msg.ID)
		return
	}

	if err := handler(ctx, []byte(data)); err != nil {
		s.logger.Error("event: handler failed, message not ACKed and will be retried",
			zap.String("msg_id", msg.ID),
			zap.String("stream", stream),
			zap.Error(err),
		)
		return // No ACK: the message stays in the PEL and is retried later.
	}

	// ACK after successful processing.
	if err := s.client.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
		s.logger.Error("event: XACK failed", zap.String("msg_id", msg.ID), zap.Error(err))
	}
}

// ensureGroup creates the consumer group idempotently.
func (s *RedisStreamSubscriber) ensureGroup(ctx context.Context, stream, group string) error {
	_, err := s.client.XGroupCreateMkStream(ctx, stream, group, "0").Result()
	if err != nil {
		// "BUSYGROUP" means the group already exists, which is expected.
		if redis.HasErrorPrefix(err, "BUSYGROUP") {
			return nil
		}
		return fmt.Errorf("event: XGROUP CREATE failed: %w", err)
	}
	return nil
}

// Close releases resources. No extra cleanup is currently required.
func (s *RedisStreamSubscriber) Close() error {
	return nil
}
