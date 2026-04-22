package natsbus

import (
	"context"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// JSPublisher 是基于 NATS JetStream 的 bus.Publisher 实现。
// 消息会被持久化到 Stream，支持 ACK 确认，保证 At-Least-Once 投递。
// 适用于任务队列、离线消息暂存、需要可靠投递的异步场景。
//
// 注意：发布前需要先创建好对应的 Stream，否则会返回 "no responders" 错误。
type JSPublisher struct {
	js jetstream.JetStream
}

// NewJSPublisher 创建 JetStream 发布者。
func NewJSPublisher(js jetstream.JetStream) *JSPublisher {
	return &JSPublisher{js: js}
}

// Publish 将 bus.Message 发布到 JetStream。
// msg.Topic 映射为 NATS Subject，必须匹配已有 Stream 的 Subject 过滤规则。
// 返回 nil 表示消息已被 JetStream 持久化确认。
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

	// PublishMsg 会等待 JetStream 的 ACK，确认消息已被持久化
	_, err := p.js.PublishMsg(ctx, natsMsg)
	return err
}

func (p *JSPublisher) Close() error {
	return nil
}
