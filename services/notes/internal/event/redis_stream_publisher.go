package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStreamPublisher 基于 Redis Stream 的事件发布实现。
//
// 使用 XADD 将消息写入 Stream，消息持久化在 Redis 中，
// 即使消费者暂时离线也不会丢失。
type RedisStreamPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisStreamPublisher 创建 Redis Stream 发布者。
func NewRedisStreamPublisher(client *redis.Client, logger *zap.Logger) *RedisStreamPublisher {
	return &RedisStreamPublisher{client: client, logger: logger}
}

// Publish 将事件发布到 Redis Stream。
//
// Stream key = topic 名称（如 "snippet.saved"）。
// 消息包含一个 "data" 字段，值为 JSON 序列化的 payload。
func (p *RedisStreamPublisher) Publish(ctx context.Context, topic Topic, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("event: 序列化 payload 失败: %w", err)
	}

	_, err = p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: string(topic),
		Values: map[string]any{
			"data": string(data),
		},
		// MaxLen + Approx: 保留最近 10000 条消息，自动裁剪旧数据，防止 Stream 无限增长
		MaxLen: 10000,
		Approx: true,
	}).Result()
	if err != nil {
		p.logger.Error("event: XADD 失败",
			zap.String("topic", string(topic)),
			zap.Error(err),
		)
		return fmt.Errorf("event: XADD 失败: %w", err)
	}

	p.logger.Debug("event: 事件已发布",
		zap.String("topic", string(topic)),
		zap.ByteString("data", data),
	)
	return nil
}

// Close 释放资源（当前无需额外清理）。
func (p *RedisStreamPublisher) Close() error {
	return nil
}
