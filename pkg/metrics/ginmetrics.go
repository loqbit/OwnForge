// Package metrics 提供统一的 Prometheus 指标采集能力。
// 本文件封装了 Gin HTTP 框架的中间件，自动记录每个请求的计数、延时和并发数。
// Prometheus 会定期来 /metrics 端点"拉取"这些数据，然后 Grafana 从 Prometheus 读取并可视化。
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// ==================== 指标定义 ====================
// Prometheus 有 3 种核心指标类型：
//   - Counter（计数器）：只增不减，适合统计"总请求数"
//   - Histogram（直方图）：记录数值分布，适合统计"请求延时"（自动算 P50/P95/P99）
//   - Gauge（仪表盘）：可增可减，适合统计"当前并发数"
//
// 每个指标可以附带"标签"（Labels），用于按 method/path/status 等维度区分。
// 最终在 Prometheus 中的完整指标名 = Namespace_Subsystem_Name，例如 gin_http_http_requests_total

var (
	// httpRequestTotal 请求总数计数器
	// 每来一个请求就 +1，按 method(GET/POST)、path(/api/v1/pastes)、status(200/404) 分组
	// 用途：计算 QPS（每秒请求数）、错误率
	httpRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	// httpRequestDuration 请求延时直方图
	// 记录每个请求花了多少秒，Prometheus 会自动计算 P50/P95/P99 等分位数
	// Buckets 定义了统计区间：5ms, 10ms, 25ms, ... 10s
	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path", "status"})

	// httpRequestInFlight 当前正在处理的请求数（并发数）
	// 请求进来 +1，处理完 -1，反映服务器当前负载
	httpRequestInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gin",
		Subsystem: "http",
		Name:      "http_requests_in_flight",
		Help:      "Number of HTTP requests currently in flight.",
	}, []string{"method", "path"})
)

// init 在包被导入时自动执行，向 Prometheus 的全局注册表注册我们定义的指标。
// 注册后 Prometheus 才能在 /metrics 端点暴露它们。
func init() {
	prometheus.MustRegister(httpRequestTotal, httpRequestDuration, httpRequestInFlight)
}

// GinMetrics 返回一个 Gin 中间件，自动为每个 HTTP 请求记录 Prometheus 指标。
//
// 工作流程：
//  1. 请求进来 → InFlight +1，记录开始时间
//  2. c.Next() → 执行后续 handler（实际业务逻辑）
//  3. 请求完成 → InFlight -1，记录总耗时和状态码
//
// 使用方式（在 router.go 中）：
//
//	r.Use(metrics.GinMetrics())
func GinMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过 /metrics 端点自身，避免 Prometheus 的 scrape 请求污染业务指标
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// 并发数 +1（请求开始）
		httpRequestInFlight.WithLabelValues(c.Request.Method, c.FullPath()).Inc()
		// defer 保证无论 handler 成功还是 panic，都会 -1（请求结束）
		defer httpRequestInFlight.WithLabelValues(c.Request.Method, c.FullPath()).Dec()

		start := time.Now()

		// 执行后续的所有 handler（业务逻辑）
		c.Next()

		// 请求结束后，记录状态码和耗时
		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		// 请求计数 +1
		httpRequestTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		// 将本次耗时"投入"到直方图的对应 bucket 中，并尝试附带 TraceID (Exemplar)
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

// GinMetricsHandler 返回一个 Gin handler，用于暴露 /metrics 端点。
// Prometheus 服务器会定期（默认 15s）来这个端点"拉取"所有已注册的指标数据。
//
// 使用方式（在 router.go 中）：
//
//	r.GET("/metrics", metrics.GinMetricsHandler())
func GinMetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
