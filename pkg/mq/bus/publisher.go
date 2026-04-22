package bus

import "context"

// Publisher defines a unified message publishing interface.
// Concrete implementations can be built on Kafka, NATS, or other message middleware.
// The business layer depends only on this abstraction and stays unaware of the underlying broker.
type Publisher interface {
	Publish(ctx context.Context, msg *Message) error
	Close() error
}
