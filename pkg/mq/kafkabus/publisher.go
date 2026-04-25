package kafkabus

import (
	"context"

	"github.com/loqbit/ownforge/pkg/mq/bus"
	"github.com/segmentio/kafka-go"
)

// Publisher is the Kafka-based implementation of bus.Publisher.
// Internally it uses kafka.Writer and supports partitioning by key automatically.
type Publisher struct {
	writer *kafka.Writer
}

// NewPublisher creates a Kafka publisher.
func NewPublisher(brokers []string) *Publisher {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		AllowAutoTopicCreation: true,
		Balancer:               &kafka.LeastBytes{},
	}
	return &Publisher{writer: w}
}

// Publish sends a bus.Message to Kafka.
// msg.Topic maps to the Kafka topic, and msg.Key is used for partition routing.
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
