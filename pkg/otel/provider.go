package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config is the common OpenTelemetry configuration.
type Config struct {
	ServiceName    string `mapstructure:"service_name"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
}

// InitTracer initializes OpenTelemetry distributed tracing.
//   - cfg.ServiceName: the current service name (for example, "api-gateway"), shown in the Jaeger UI
//   - cfg.JaegerEndpoint: the Jaeger OTLP collector address (for example, "localhost:4318"), without a protocol prefix
//   - returns a shutdown function to call on exit from main, flushing any buffered spans
func InitTracer(cfg Config) (func(context.Context) error, error) {
	ctx := context.Background()

	// 1. Create the exporter
	// It sends collected span data to Jaeger over OTLP HTTP.
	// WithEndpoint only needs host:port, without the http:// prefix.
	// WithInsecure uses plain HTTP for development; production should use TLS.
	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(cfg.JaegerEndpoint),
		otlptracehttp.WithInsecure(),
	))
	if err != nil {
		return nil, err
	}

	// 2. Create the resource metadata
	// This tells Jaeger which service the span data comes from.
	// semconv.ServiceName is the standard attribute defined by OpenTelemetry semantic conventions
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
	))
	if err != nil {
		return nil, err
	}

	// 3. Create the TracerProvider
	// It assembles the exporter and resource, forming the core of the tracing system.
	// WithBatcher sends spans in batches, which is more efficient than sending them one by one.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// 4. Register it as the global TracerProvider
	// After this, any call to otel.Tracer("xxx") will use this provider.
	otel.SetTracerProvider(tp)

	// 5. Set the global propagator
	// TraceContext is the W3C standard for propagating TraceID and SpanID across services via the traceparent HTTP header.
	// This lets spans created by the gateway and downstream services form one complete trace.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return the shutdown function so callers can defer shutdown(ctx) when the program exits.
	return tp.Shutdown, nil
}
