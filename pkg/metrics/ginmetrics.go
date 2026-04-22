// Package metrics provides unified Prometheus metrics collection.
// This file wraps Gin HTTP middleware to automatically record request counts, latency, and concurrency.
// Prometheus periodically scrapes these metrics from /metrics, and Grafana reads and visualizes them from Prometheus.
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// ==================== Metric definitions ====================
// Prometheus has three core metric types:
//   - Counter: monotonically increasing, suitable for total request counts
//   - Histogram: records value distributions, suitable for request latency (for example P50/P95/P99)
//   - Gauge: can increase or decrease, suitable for current concurrency
//
// Each metric can include labels to distinguish dimensions such as method, path, and status.
// The full metric name in Prometheus is Namespace_Subsystem_Name, for example gin_http_http_requests_total.

var (
	// httpRequestTotal is the total request counter
	// It increments by 1 for every request, grouped by method (GET/POST), path, and status (200/404).
	// Use it to calculate QPS and error rates.
	httpRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	// httpRequestDuration is the request latency histogram
	// It records how many seconds each request takes, and Prometheus can calculate P50/P95/P99 automatically.
	// Buckets define histogram ranges: 5ms, 10ms, 25ms, ... 10s.
	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path", "status"})

	// httpRequestInFlight is the number of requests currently being processed (in-flight count)
	// Increment on request arrival and decrement on completion to reflect current server load.
	httpRequestInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_requests_in_flight",
		Help:      "Number of HTTP requests currently in flight.",
	}, []string{"method", "path"})
)

// init runs automatically when the package is imported and registers the metrics with the global Prometheus registry.
// Once registered, Prometheus can expose them at /metrics.
func init() {
	prometheus.MustRegister(httpRequestTotal, httpRequestDuration, httpRequestInFlight)
}

// GinMetrics returns a Gin middleware that automatically records Prometheus metrics for each HTTP request.
//
// Flow:
//  1. Request arrives -> InFlight +1 and record the start time
//  2. c.Next() -> execute downstream handlers (actual business logic)
//  3. Request completes -> InFlight -1, then record total latency and status code
//
// Usage (in router.go):
//
//	r.Use(metrics.GinMetrics())
func GinMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip the /metrics endpoint itself so Prometheus scrapes do not pollute business metrics.
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Increment in-flight count at request start
		httpRequestInFlight.WithLabelValues(c.Request.Method, c.FullPath()).Inc()
		// defer ensures the counter is decremented when the request ends, whether the handler succeeds or panics
		defer httpRequestInFlight.WithLabelValues(c.Request.Method, c.FullPath()).Dec()

		start := time.Now()

		// Execute all downstream handlers (business logic).
		c.Next()

		// Record status code and latency after the request finishes.
		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		// Increment the request counter by 1.
		httpRequestTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		// Observe this latency in the appropriate histogram bucket and try to attach a TraceID exemplar.
		observer := httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath(), status)
		span := trace.SpanFromContext(c.Request.Context())
		if span.SpanContext().IsValid() {
			if exemplarObserver, ok := observer.(prometheus.ExemplarObserver); ok {
				exemplarObserver.ObserveWithExemplar(duration, prometheus.Labels{"trace_id": span.SpanContext().TraceID().String()})
			} else {
				observer.Observe(duration)
			}
		} else {
			observer.Observe(duration)
		}
	}
}

// GinMetricsHandler returns a Gin handler that exposes /metrics.
// The Prometheus server scrapes this endpoint periodically (15s by default) to collect all registered metrics.
//
// Usage (in router.go):
//
//	r.GET("/metrics", metrics.GinMetricsHandler())
func GinMetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
