package natsbus

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/ownforge/ownforge/pkg/mq/bus"
)

// Publisher is the Core NATS implementation of bus.Publisher.
// It is fire-and-forget and does not guarantee persistence.
// It is suitable for real-time notifications, chat-message routing, cache-invalidation broadcasts, and similar scenarios.
type Publisher struct {
	conn *nats.Conn
}

// NewPublisher creates a Core NATS publisher.
func NewPublisher(conn *nats.Conn) *Publisher {
	return &Publisher{conn: conn}
}

// Publish publishes a bus.Message to a NATS subject.
// msg.Topic maps to the NATS subject, while msg.Key and msg.Headers are carried through NATS headers.
func (p *Publisher) Publish(ctx context.Context, msg *bus.Message) error {
	natsMsg := &nats.Msg{
		Subject: msg.Topic,
		Data:    msg.Value,
		Header:  make(nats.Header),
	}

	// Put bus.Message.Key into the NATS header to keep semantics aligned with Kafka.
	if msg.Key != "" {
		natsMsg.Header.Set(HeaderKey, msg.Key)
	}

	// Pass through business headers.
	for k, v := range msg.Headers {
		natsMsg.Header.Set(k, string(v))
	}

	return p.conn.PublishMsg(natsMsg)
}

func (p *Publisher) Close() error {
	// The Core NATS connection lifecycle is usually managed by the caller, so it is not closed here.
	// Call Flush if the buffer needs to be flushed.
	return p.conn.Flush()
}
