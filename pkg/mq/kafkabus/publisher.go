package kafkabus

import (
	"context"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/segmentio/kafka-go"
)

// Publisher 是基于 Kafka 的 bus.Publisher 实现。
// 内部使用 kafka.Writer 发送消息，支持自动按 Key 分 Partition。
type Publisher struct {
	writer *kafka.Writer
}

// NewPublisher 创建 Kafka 发布者。
func NewPublisher(brokers []string) *Publisher {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		AllowAutoTopicCreation: true,
		Balancer:               &kafka.LeastBytes{},
	}
	return &Publisher{writer: w}
}

// Publish 将 bus.Message 发送到 Kafka。
// msg.Topic 映射为 Kafka Topic，msg.Key 用于 Partition 路由。
func (p *Publisher) Publish(ctx context.Context, msg *bus.Message) error {
	headers := make([]kafka.Header, 0, len(msg.Headers))
	for k, v := range msg.Headers {
		headers = append(headers, kafka.Header{Key: k, Value: v})
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   msg.Topic,
		Key:     []byte(msg.Key),
		Value:   msg.Value,
		Headers: headers,
	})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}
