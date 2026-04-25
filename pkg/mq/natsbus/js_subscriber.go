package natsbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/loqbit/ownforge/pkg/mq/bus"
)

// JSSubscriber is the JetStream-based implementation of bus.Subscriber.
// It supports durable consumption, ACK/NAK confirmation, and Queue Group load balancing.
// It is suitable for task queues, offline messages, and other scenarios that need reliable delivery.
type JSSubscriber struct {
	js      jetstream.JetStream
	stream  string // stream name (JetStream storage unit)
	subject string // subject filter to consume
	durable string // durable consumer name; resumes from the last position after restart
	queue   string // DeliverGroup for multi-instance load balancing

	mu      sync.Mutex
	consume jetstream.ConsumeContext
}

// NewJSSubscriber creates a JetStream subscriber.
//
// stream is the JetStream stream name and must already exist.
// subject is the subject filter to consume.
//
// Example:
//
//	// durable consumption with Queue Group load balancing
//	sub := NewJSSubscriber(js, "TASKS", "task.>",
//	    WithJSDurable("task-worker"),
//	    WithQueue("workers"),
//	)
func NewJSSubscriber(js jetstream.JetStream, stream, subject string, opts ...Option) *JSSubscriber {
	s := &JSSubscriber{
		js:      js,
		stream:  stream,
		subject: subject,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// Start begins consuming JetStream messages and blocks until ctx is canceled.
// If handler returns nil -> ACK the message; if it returns an error -> NAK and retry later.
func (s *JSSubscriber) Start(ctx context.Context, handler bus.Handler) error {
	// build the consumer configuration
	consumerCfg := jetstream.ConsumerConfig{
		FilterSubject: s.subject,
	}
	if s.durable != "" {
		consumerCfg.Durable = s.durable
	}
	if s.queue != "" {
		consumerCfg.DeliverGroup = s.queue
	}

	// create or update the consumer (idempotent)
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.stream, consumerCfg)
	if err != nil {
		return fmt.Errorf("natsbus: failed to create JetStream consumer: %w", err)
	}

	// start consuming
	cc, err := consumer.Consume(func(m jetstream.Msg) {
		msg := jsToMessage(m)
		if err := handler.Handle(ctx, msg); err != nil {
			// on failure, NAK so JetStream can retry later
			_ = m.Nak()
			return
		}
		// on success, ACK the message
		_ = m.Ack()
	})
	if err != nil {
		return fmt.Errorf("natsbus: failed to start JetStream consumption: %w", err)
	}

	s.mu.Lock()
	s.consume = cc
	s.mu.Unlock()

	// block until a cancellation signal arrives
	<-ctx.Done()
	cc.Stop()
	return nil
}

func (s *JSSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consume != nil {
		s.consume.Stop()
	}
	return nil
}

// jsToMessage converts a JetStream message into bus.Message.
func jsToMessage(m jetstream.Msg) *bus.Message {
	natsHeaders := m.Headers()
	headers := make(map[string][]byte, len(natsHeaders))
	for k, vals := range natsHeaders {
		if len(vals) > 0 {
			headers[k] = []byte(vals[0])
		}
	}

	key := ""
	if v := natsHeaders.Get(HeaderKey); v != "" {
		key = v
		delete(headers, HeaderKey)
	}

	return &bus.Message{
		Topic:   m.Subject(),
		Key:     key,
		Value:   m.Data(),
		Headers: headers,
		Metadata: map[string]string{
			"transport": "nats-jetstream",
			"stream":    m.Headers().Get("Nats-Stream"),
		},
	}
}
