// Package metrics 提供统一的 Prometheus 指标采集能力。
// 本文件封装了 gRPC 服务端的拦截器（Interceptor），自动记录每个 RPC 调用的计数、延时和并发数。
//
// 因为 gRPC 服务本身不提供 HTTP 端点，所以额外提供了 ServeMetrics() 函数，
// 在一个独立的 HTTP 端口上暴露 /metrics，供 Prometheus 来拉取。
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

// ==================== 指标定义 ====================
// 与 ginmetrics.go 中的 HTTP 指标对应，这里定义了 gRPC 版本的三大指标。
// 标签中的 method 是 gRPC 的 FullMethod，格式为 "/包名.服务名/方法名"
// 标签中的 status 是 gRPC 状态码，如 "OK"、"NotFound"、"Internal"

var (
	// grpcRequestTotal gRPC 请求总数计数器
	// 每完成一个 RPC 调用就 +1，按 method 和 status code 分组
	grpcRequestTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests.",
	}, []string{"method", "status"})

	// grpcRequestDuration gRPC 请求延时直方图
	// 记录每次 RPC 调用的耗时，Prometheus 自动计算 P50/P95/P99
	grpcRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "status"})

	// grpcRequestInFlight 当前正在处理的 gRPC 请求数
	grpcRequestInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "grpc",
		Subsystem: "rpc",
		Name:      "grpc_requests_in_flight",
		Help:      "Number of gRPC requests currently in flight.",
	}, []string{"method"})
)

// init 包初始化时向 Prometheus 全局注册表注册 gRPC 指标
func init() {
	prometheus.MustRegister(grpcRequestTotal, grpcRequestDuration, grpcRequestInFlight)
}

// GRPCMetricsInterceptor 返回一个 gRPC 一元拦截器（UnaryServerInterceptor）。
// 拦截器就是 gRPC 版的"中间件"，在真正执行 RPC 方法前后插入逻辑。
//
// 工作流程：
//  1. RPC 请求进来 → InFlight +1
//  2. handler(ctx, req) → 执行实际的 RPC 方法（如 GetUser、Login）
//  3. RPC 完成 → InFlight -1，记录状态码和耗时
//
// 使用方式（在 grpc main.go 中）：
//
//	s := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        metrics.GRPCMetricsInterceptor(),  // 放在拦截器链的第一个
//	        interceptor.RecoveryInterceptor(log),
//	        ...
//	    ),
//	)
func GRPCMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 并发数 +1（请求开始）
		grpcRequestInFlight.WithLabelValues(info.FullMethod).Inc()
		// defer 保证请求结束后 -1
		defer grpcRequestInFlight.WithLabelValues(info.FullMethod).Dec()

		start := time.Now()

		// 调用实际的 gRPC handler（业务逻辑）
		resp, err := handler(ctx, req)

		// 从 error 中提取 gRPC 状态码（OK / NotFound / Internal 等）
		code := status.Code(err).String()
		dur := time.Since(start).Seconds()

		// 记录请求计数和延时，尝试注入 TraceID Exemplar
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

// ServeMetrics 在指定地址启动一个独立的 HTTP 服务器，仅暴露 /metrics 端点。
//
// 为什么需要这个？
// gRPC 服务运行在纯 TCP 协议上，没有 HTTP 路由的概念，
// 但 Prometheus 必须通过 HTTP GET /metrics 来拉取指标。
// 所以我们额外起一个轻量 HTTP server，专门给 Prometheus 用。
//
// 使用方式（在 grpc main.go 中，用 goroutine 启动）：
//
//	go metrics.ServeMetrics(":9092")  // 在 9092 端口暴露指标
func ServeMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(addr, mux)
}
