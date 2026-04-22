package busmiddleware

import (
	"context"
	"encoding/json"

	"github.com/ownforge/ownforge/pkg/mq/bus"
	commontrace "github.com/ownforge/ownforge/pkg/trace"
)

const defaultOutboxHeadersKey = "x-outbox-headers"

// WithTrace 会尝试从消息头中提取 trace_id，并重新注入到 ctx。
// 默认支持：
// 1. 直接从 HeaderTraceID 读取，如 x-trace-id
// 2. 从 Debezium Outbox 的 x-outbox-headers JSON 中读取
func WithTrace() Middleware {
	return func(next bus.Handler) bus.Handler {
		return bus.HandlerFunc(func(ctx context.Context, msg *bus.Message) error {
			traceID := traceIDFromMessage(msg)
			if traceID != "" {
				ctx = commontrace.IntoContext(ctx, traceID)
			}
			return next.Handle(ctx, msg)
		})
	}
}

func traceIDFromMessage(msg *bus.Message) string {
	if msg == nil || len(msg.Headers) == 0 {
		return ""
	}

	if traceID := string(msg.Headers[commontrace.HeaderTraceID]); traceID != "" {
		return traceID
	}

	raw := msg.Headers[defaultOutboxHeadersKey]
	if len(raw) == 0 {
		return ""
	}

	var headers map[string]string
	if err := json.Unmarshal(raw, &headers); err != nil {
		return ""
	}
	return headers[commontrace.HeaderTraceID]
}
