package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/ownforge/ownforge/pkg/trace"
)

// TraceMiddleware injects and propagates the request trace ID.
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Try to read the trace ID from the upstream service or gateway header.
		traceID := c.GetHeader(trace.HeaderTraceID)
		if traceID == "" {
			// 2. Generate a new trace ID when the request does not have one.
			traceID = trace.NewTraceID()
		}

		// 3. Put the trace ID back into the current response headers for downstream consumers.
		c.Header(trace.HeaderTraceID, traceID)

		// 4. Store the trace ID in the standard context so later service and DB layers can read it.
		ctx := trace.IntoContext(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
