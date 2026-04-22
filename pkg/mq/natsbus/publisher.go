package natsbus

import (
	"context"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/nats-io/nats.go"
)

// Publisher 是基于 NATS Core 的 bus.Publisher 实现。
// 消息发出即忘（fire-and-forget），不保证持久化。
// 适用于实时通知、聊天消息路由、缓存失效广播等场景。
type Publisher struct {
	conn *nats.Conn
}

// NewPublisher 创建 Core NATS 发布者。
func NewPublisher(conn *nats.Conn) *Publisher {
	return &Publisher{conn: conn}
}

// Publish 将 bus.Message 发布到 NATS Subject。
// msg.Topic 映射为 NATS Subject，msg.Key 和 msg.Headers 通过 NATS Header 传递。
func (p *Publisher) Publish(ctx context.Context, msg *bus.Message) error {
	natsMsg := &nats.Msg{
		Subject: msg.Topic,
		Data:    msg.Value,
		Header:  make(nats.Header),
	}

	// 把 bus.Message 的 Key 放到 NATS Header 里，保持和 Kafka 的语义一致
	if msg.Key != "" {
		natsMsg.Header.Set(HeaderKey, msg.Key)
	}

	// 透传业务 Headers
	for k, v := range msg.Headers {
		natsMsg.Header.Set(k, string(v))
	}

	return p.conn.PublishMsg(natsMsg)
}

func (p *Publisher) Close() error {
	// Core NATS 的 conn 通常由调用方管理生命周期，这里不主动关闭。
	// 如果需要刷新缓冲区，调用 Flush。
	return p.conn.Flush()
}
