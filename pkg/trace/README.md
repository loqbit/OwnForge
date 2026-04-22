# common/trace — Request Trace ID

A lightweight tool for TraceID generation and context propagation.

> **Note**: This package is for custom TraceID scenarios. In most cases, `common/otel` automatic OpenTelemetry tracing is enough.

## Usage

```go
import "github.com/ownforge/ownforge/pkg/trace"

// Generate a new TraceID
traceID := trace.NewTraceID()

// Store in context
ctx = trace.IntoContext(ctx, traceID)

// Extract from context
id := trace.FromContext(ctx)
```
