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

// Config 通用 OpenTelemetry 配置
type Config struct {
	ServiceName    string `mapstructure:"service_name"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
}

// InitTracer 初始化 OpenTelemetry 分布式追踪
//   - cfg.ServiceName: 当前服务名称（如 "api-gateway"），会在 Jaeger UI 中显示
//   - cfg.JaegerEndpoint: Jaeger OTLP 接收器地址（如 "localhost:4318"），不带协议前缀
//   - 返回 shutdown 函数：在 main 退出时调用，flush 缓冲区中未发送的 Span 数据
func InitTracer(cfg Config) (func(context.Context) error, error) {
	ctx := context.Background()

	// 1. 创建 Exporter（导出器）
	// 负责把采集到的 Span 数据通过 OTLP HTTP 协议发送给 Jaeger
	// WithEndpoint 只需要 host:port，不带 http:// 前缀
	// WithInsecure 表示用明文 HTTP（开发环境），生产环境应该用 TLS
	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(cfg.JaegerEndpoint),
		otlptracehttp.WithInsecure(),
	))
	if err != nil {
		return nil, err
	}

	// 2. 创建 Resource（资源标识）
	// 告诉 Jaeger「这些 Span 数据来自哪个服务」
	// semconv.ServiceName 是 OpenTelemetry 语义约定中定义的标准属性
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
	))
	if err != nil {
		return nil, err
	}

	// 3. 创建 TracerProvider（追踪器提供者）
	// 组装 Exporter + Resource，是整个追踪系统的核心
	// WithBatcher: 批量发送 Span，比逐条发送更高效（内部有缓冲队列和定时刷新）
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// 4. 注册为全局 TracerProvider
	// 之后任何地方调用 otel.Tracer("xxx") 都会使用这个 Provider
	otel.SetTracerProvider(tp)

	// 5. 设置全局 Propagator（传播器）
	// TraceContext 是 W3C 标准，通过 HTTP Header（traceparent）跨服务传递 TraceID 和 SpanID
	// 这样网关创建的 Span 和下游服务的 Span 就能串成一条完整的链路
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// 返回 shutdown 函数，调用方在程序退出时执行 defer shutdown(ctx)
	return tp.Shutdown, nil
}

