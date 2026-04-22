package kafkabus

import (
	"context"
	"fmt"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/segmentio/kafka-go"
)

// Subscriber 是基于 Kafka 的 bus.Subscriber 实现。
// 内部使用 kafka.Reader（Consumer Group 模式）拉取消息。
// handler 返回 nil → 提交 Offset；返回 error → 不提交，下次重新消费。
type Subscriber struct {
	reader *kafka.Reader
}

// NewSubscriber 创建 Kafka 订阅者。
func NewSubscriber(brokers []string, topic, groupID string) *Subscriber {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic:   topic,
	})
	return &Subscriber{reader: r}
}

// Start 开始消费 Kafka 消息，阻塞直到 ctx 取消。
// 逻辑从 email-message/internal/handler/consumer.go 提取而来。
func (s *Subscriber) Start(ctx context.Context, handler bus.Handler) error {
	for {
		m, err := s.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // 正常退出
			}
			continue
		}

		msg := kafkaToMessage(m)

		if err := handler.Handle(ctx, msg); err != nil {
			continue // 不提交 Offset，下次重新消费
		}

		// 处理成功，手动提交 Offset
		_ = s.reader.CommitMessages(ctx, m)
	}
}

func (s *Subscriber) Close() error {
	return s.reader.Close()
}

// kafkaToMessage 将 Kafka 消息转换为 bus.Message。
func kafkaToMessage(m kafka.Message) *bus.Message {
	headers := make(map[string][]byte, len(m.Headers))
	for _, h := range m.Headers {
		headers[h.Key] = h.Value
	}

	return &bus.Message{
		Topic:   m.Topic,
		Key:     string(m.Key),
		Value:   m.Value,
		Headers: headers,
		Metadata: map[string]string{
			"transport": "kafka",
			"offset":    fmt.Sprintf("%d:%d", m.Partition, m.Offset),
		},
	}
}
