package event

import "context"

// Handler processes one event. Returning nil marks it successful and ACKs it; returning an error leaves it pending for retry.
type Handler func(ctx context.Context, data []byte) error

// Subscriber abstracts event subscriptions.
//
// Current implementation: RedisStreamSubscriber based on Redis Stream consumer groups.
// In a future phase, this can be replaced by MemorySubscriber based on Go channels.
type Subscriber interface {
	// Subscribe consumes the given topic through a consumer group.
	//   - group: consumer group name; multiple consumers in the same group do not process the same message
	//   - consumer: current consumer instance name, used to distinguish workers in the same group
	//   - handler: message handler
	//
	// This call blocks until ctx is canceled.
	Subscribe(ctx context.Context, topic Topic, group, consumer string, handler Handler) error
	// Close releases resources.
	Close() error
}
