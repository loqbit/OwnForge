// Package metrics provides unified Prometheus metrics collection.
// This file wraps gRPC server interceptors to automatically record RPC counts, latency, and concurrency.
//
// Because gRPC services do not provide HTTP endpoints themselves, this file also provides ServeMetrics(),
// which exposes /metrics on a separate HTTP port for Prometheus to scrape.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// ==================== Metric definitions ====================
// These are the gRPC counterparts to the HTTP metrics in ginmetrics.go.
// The method label is the gRPC FullMethod, in the format "/package.service/method".
// The status label is the gRPC status code, such as "OK", "NotFound", or "Internal".

var (
	// grpcRequestTotal gRPC request total counter
	// It increments by 1 for every completed RPC, grouped by method and status code.
	grpcRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests.",
	}, []string{"method", "status"})

	// grpcRequestDuration gRPC request latency histogram
	// It records RPC latency so Prometheus can compute P50/P95/P99.
	grpcRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "status"})

	// grpcRequestInFlight is the current number of in-flight gRPC requests
	grpcRequestInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_requests_in_flight",
		Help:      "Number of gRPC requests currently in flight.",
	}, []string{"method"})
)

// The init function registers gRPC metrics with the global Prometheus registry.
func init() {
	prometheus.MustRegister(grpcRequestTotal, grpcRequestDuration, grpcRequestInFlight)
}

// GRPCMetricsInterceptor returns a gRPC unary interceptor.
// An interceptor is the gRPC equivalent of middleware, inserting logic before and after the RPC method runs.
//
// Flow:
//  1. RPC Request arrives -> InFlight +1
//  2. handler(ctx, req) -> execute the actual RPC method, such as GetUser or Login
//  3. RPC completes -> InFlight -1, then record status code and latency
//
// Usage (in grpc main.go):
//
//	s := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        metrics.GRPCMetricsInterceptor(),  // place it first in the interceptor chain
//	        interceptor.RecoveryInterceptor(log),
//	        ...
//	    ),
//	)
func GRPCMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Increment in-flight count at request start
		grpcRequestInFlight.WithLabelValues(info.FullMethod).Inc()
		// defer guarantees it is decremented when the request ends
		defer grpcRequestInFlight.WithLabelValues(info.FullMethod).Dec()

		start := time.Now()

		// Call the actual gRPC handler (business logic)
		resp, err := handler(ctx, req)

		// Extract the gRPC status code from the error (OK / NotFound / Internal, etc.).
		code := status.Code(err).String()
		dur := time.Since(start).Seconds()

		// Record request counts and latency, and try to inject a TraceID exemplar.
		grpcRequestTotal.WithLabelValues(info.FullMethod, code).Inc()

		observer := grpcRequestDuration.WithLabelValues(info.FullMethod, code)
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			if exemplarObserver, ok := observer.(prometheus.ExemplarObserver); ok {
				exemplarObserver.ObserveWithExemplar(dur, prometheus.Labels{"trace_id": span.SpanContext().TraceID().String()})
			} else {
				observer.Observe(dur)
			}
		} else {
			observer.Observe(dur)
		}

		return resp, err
	}
}

// ServeMetrics starts a standalone HTTP server at the given address and exposes only /metrics.
//
// Why is this needed?
// gRPC services run over raw TCP and do not have HTTP routing,
// but Prometheus must scrape metrics through HTTP GET /metrics.
// So we start an extra lightweight HTTP server specifically for Prometheus.
//
// Usage (start it in grpc main.go using a goroutine):
//
//	go metrics.ServeMetrics(":9092")  // expose metrics on port 9092
func ServeMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(addr, mux)
}
