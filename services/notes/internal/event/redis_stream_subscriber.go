package event

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisStreamSubscriber 基于 Redis Stream 消费者组的事件订阅实现。
//
// 核心流程：
//  1. 启动时自动创建消费者组（XGROUP CREATE ... MKSTREAM）
//  2. 循环 XREADGROUP 拉取新消息
//  3. handler 处理成功 → XACK 确认
//  4. handler 处理失败 → 不 ACK，消息留在 Pending Entries List，下次启动自动重试
type RedisStreamSubscriber struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisStreamSubscriber 创建 Redis Stream 订阅者。
func NewRedisStreamSubscriber(client *redis.Client, logger *zap.Logger) *RedisStreamSubscriber {
	return &RedisStreamSubscriber{client: client, logger: logger}
}

// Subscribe 以消费者组模式订阅 topic，阻塞直到 ctx 取消。
func (s *RedisStreamSubscriber) Subscribe(ctx context.Context, topic Topic, group, consumer string, handler Handler) error {
	stream := string(topic)

	// 1. 创建消费者组（幂等：已存在则忽略）
	if err := s.ensureGroup(ctx, stream, group); err != nil {
		return err
	}

	s.logger.Info("event: 开始订阅",
		zap.String("topic", stream),
		zap.String("group", group),
		zap.String("consumer", consumer),
	)

	// 2. 先处理 Pending 消息（上次未 ACK 的），再处理新消息
	if err := s.processPending(ctx, stream, group, consumer, handler); err != nil {
		s.logger.Warn("event: 处理 Pending 消息时出错", zap.Error(err))
	}

	// 3. 循环读取新消息
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("event: 订阅已停止", zap.String("topic", stream))
			return ctx.Err()
		default:
		}

		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},  // ">" 表示只读新消息
			Count:    10,                       // 每批最多拉 10 条
			Block:    3 * time.Second,          // 阻塞等待 3 秒
		}).Result()

		if err != nil {
			if err == redis.Nil || err == context.Canceled || err == context.DeadlineExceeded {
				continue // 超时无消息或 ctx 取消，继续
			}
			s.logger.Error("event: XREADGROUP 失败", zap.Error(err))
			time.Sleep(time.Second) // 避免错误风暴
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				s.handleMessage(ctx, stream.Stream, group, msg, handler)
			}
		}
	}
}

// processPending 处理上次未 ACK 的 Pending 消息（Worker 重启后的断点续消费）。
func (s *RedisStreamSubscriber) processPending(ctx context.Context, stream, group, consumer string, handler Handler) error {
	for {
		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, "0"}, // "0" 表示读取 Pending 消息
			Count:    10,
		}).Result()
		if err != nil {
			return fmt.Errorf("event: 读取 Pending 消息失败: %w", err)
		}

		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			return nil // 没有 Pending 消息了
		}

		for _, msg := range streams[0].Messages {
			s.handleMessage(ctx, stream, group, msg, handler)
		}
	}
}

// handleMessage 处理单条消息：调用 handler，成功则 ACK。
func (s *RedisStreamSubscriber) handleMessage(ctx context.Context, stream, group string, msg redis.XMessage, handler Handler) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		s.logger.Warn("event: 消息缺少 data 字段, 自动 ACK 跳过",
			zap.String("msg_id", msg.ID),
		)
		s.client.XAck(ctx, stream, group, msg.ID)
		return
	}

	if err := handler(ctx, []byte(data)); err != nil {
		s.logger.Error("event: handler 处理失败，消息不 ACK（将重试）",
			zap.String("msg_id", msg.ID),
			zap.String("stream", stream),
			zap.Error(err),
		)
		return // 不 ACK → 消息留在 PEL → 下次重试
	}

	// 处理成功，ACK 确认
	if err := s.client.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
		s.logger.Error("event: XACK 失败", zap.String("msg_id", msg.ID), zap.Error(err))
	}
}

// ensureGroup 创建消费者组（幂等操作）。
func (s *RedisStreamSubscriber) ensureGroup(ctx context.Context, stream, group string) error {
	_, err := s.client.XGroupCreateMkStream(ctx, stream, group, "0").Result()
	if err != nil {
		// "BUSYGROUP" 表示组已存在，正常忽略
		if redis.HasErrorPrefix(err, "BUSYGROUP") {
			return nil
		}
		return fmt.Errorf("event: XGROUP CREATE 失败: %w", err)
	}
	return nil
}

// Close 释放资源（当前无需额外清理）。
func (s *RedisStreamSubscriber) Close() error {
	return nil
}
