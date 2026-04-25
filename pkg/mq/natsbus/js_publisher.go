package natsbus

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/loqbit/ownforge/pkg/mq/bus"
)

// JSPublisher is the JetStream implementation of bus.Publisher.
// Messages are persisted to a stream, support ACK confirmation, and provide at-least-once delivery.
// It is suitable for task queues, offline message buffering, and asynchronous scenarios that require reliable delivery.
//
// Note: the target stream must be created before publishing, otherwise NATS returns "no responders".
type JSPublisher struct {
	js jetstream.JetStream
}

// NewJSPublisher creates a JetStream publisher.
func NewJSPublisher(js jetstream.JetStream) *JSPublisher {
	return &JSPublisher{js: js}
}

// Publish publishes a bus.Message to JetStream.
// msg.Topic maps to a NATS subject and must match the existing stream's subject filter.
// Returning nil means the message has been durably acknowledged by JetStream.
func (p *JSPublisher) Publish(ctx context.Context, msg *bus.Message) error {
	natsMsg := &nats.Msg{
		Subject: msg.Topic,
		Data:    msg.Value,
		Header:  make(nats.Header),
	}

	if msg.Key != "" {
		natsMsg.Header.Set(HeaderKey, msg.Key)
	}

	for k, v := range msg.Headers {
		natsMsg.Header.Set(k, string(v))
	}

	// PublishMsg waits for the JetStream ACK to confirm the message is persisted.
	_, err := p.js.PublishMsg(ctx, natsMsg)
	return err
}

func (p *JSPublisher) Close() error {
	return nil
}
