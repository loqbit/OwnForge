package trace

import (
	"context"

	"github.com/google/uuid"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	TraceIDKey    contextKey = "X-Trace-Id"
	HeaderTraceID string     = "x-trace-id"
)

func NewTraceID() string {
	return uuid.NewString()
}
func IntoContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	if val, ok := ctx.Value(TraceIDKey).(string); ok {
		return val
	}
	return ""
}
