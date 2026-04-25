package busmiddleware

import (
	"context"
	"encoding/json"

	"github.com/loqbit/ownforge/pkg/mq/bus"
	commontrace "github.com/loqbit/ownforge/pkg/trace"
)

const defaultOutboxHeadersKey = "x-outbox-headers"

// WithTrace tries to extract trace_id from message headers and inject it back into ctx.
// Supported by default:
// 1. reading HeaderTraceID directly, such as x-trace-id
// 2. reading from the x-outbox-headers JSON produced by Debezium Outbox
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
