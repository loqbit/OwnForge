package natsbus

import (
	"context"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/ownforge/ownforge/pkg/mq/bus"
)

// Subscriber is the Core NATS implementation of bus.Subscriber.
// It is pure in-memory Pub/Sub with no persistence, suitable for online real-time scenarios.
// It supports Queue Groups for multi-instance load balancing.
type Subscriber struct {
	conn    *nats.Conn
	subject string
	queue   string // empty means a normal subscription; non-empty means a Queue Group subscription

	mu  sync.Mutex
	sub *nats.Subscription
}

// NewSubscriber creates a Core NATS subscriber.
// subject supports NATS wildcards: * matches one level, > matches multiple levels.
//
// Example:
//
//	// subscribe to all chat-room messages
//	sub := NewSubscriber(conn, "chat.room.>")
//
//	// load-balanced subscription with a Queue Group
//	sub := NewSubscriber(conn, "task.>", WithQueue("workers"))
func NewSubscriber(conn *nats.Conn, subject string, opts ...Option) *Subscriber {
	s := &Subscriber{
		conn:    conn,
		subject: subject,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// Start begins subscribing to messages and blocks until ctx is canceled.
// Each message is converted to bus.Message before being passed to the handler.
// Core NATS has no ACK mechanism, so handler errors are only for logging and monitoring.
func (s *Subscriber) Start(ctx context.Context, handler bus.Handler) error {
	msgHandler := func(m *nats.Msg) {
		msg := coreToMessage(m)
		// Core NATS has no ack/nack, so handler errors are handled by upper layers.
		_ = handler.Handle(ctx, msg)
	}

	var (
		sub *nats.Subscription
		err error
	)
	if s.queue != "" {
		sub, err = s.conn.QueueSubscribe(s.subject, s.queue, msgHandler)
	} else {
		sub, err = s.conn.Subscribe(s.subject, msgHandler)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.sub = sub
	s.mu.Unlock()

	// block until a cancellation signal arrives
	<-ctx.Done()

	// Graceful exit: Drain finishes already received messages before unsubscribing.
	return sub.Drain()
}

func (s *Subscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sub != nil {
		return s.sub.Drain()
	}
	return nil
}

// coreToMessage converts a Core NATS message into bus.Message.
func coreToMessage(m *nats.Msg) *bus.Message {
	headers := make(map[string][]byte, len(m.Header))
	for k, vals := range m.Header {
		if len(vals) > 0 {
			headers[k] = []byte(vals[0])
		}
	}

	// extract Key from the header if present
	key := ""
	if v := m.Header.Get(HeaderKey); v != "" {
		key = v
		delete(headers, HeaderKey)
	}

	return &bus.Message{
		Topic:   m.Subject,
		Key:     key,
		Value:   m.Data,
		Headers: headers,
		Metadata: map[string]string{
			"transport": "nats-core",
		},
	}
}
