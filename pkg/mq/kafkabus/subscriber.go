package kafkabus

import (
	"context"
	"fmt"

	"github.com/loqbit/ownforge/pkg/mq/bus"
	"github.com/segmentio/kafka-go"
)

// Subscriber is the Kafka implementation of bus.Subscriber.
// Internally it uses kafka.Reader in consumer-group mode to pull messages.
// If handler returns nil, commit the offset; if it returns an error, do not commit and let it be consumed again later.
type Subscriber struct {
	reader *kafka.Reader
}

// NewSubscriber creates a Kafka subscriber.
func NewSubscriber(brokers []string, topic, groupID string) *Subscriber {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic:   topic,
	})
	return &Subscriber{reader: r}
}

// Start begins consuming Kafka messages and blocks until ctx is canceled.
// This logic was extracted from email-message/internal/handler/consumer.go.
func (s *Subscriber) Start(ctx context.Context, handler bus.Handler) error {
	for {
		m, err := s.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // normal exit
			}
			continue
		}

		msg := kafkaToMessage(m)

		if err := handler.Handle(ctx, msg); err != nil {
			continue // do not commit the offset so it can be consumed again later
		}

		// on success, commit the offset manually
		_ = s.reader.CommitMessages(ctx, m)
	}
}

func (s *Subscriber) Close() error {
	return s.reader.Close()
}

// kafkaToMessage converts a Kafka message into bus.Message.
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
