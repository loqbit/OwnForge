package logger

import (
	"context"
	"os"

	"github.com/ownforge/ownforge/pkg/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// 创建一个新的 Logger 实例
// serviceName 用于标识日志来源的服务名称
func NewLogger(serviceName string) *zap.Logger {
	config := zapcore.EncoderConfig{
		TimeKey:       "timestamp",
		LevelKey:      "level",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "stacktrace",

		EncodeTime:   zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	// 开发环境使用Debug级别，生产环境使用Info级别
	level := zapcore.InfoLevel
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENV")
	}

	if env == "dev" || env == "development" {
		level = zapcore.DebugLevel
		// 开发环境加点颜色高亮，方便人眼阅读
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// 选择编码器：容器环境用 JSON，本地用 Console
	isContainer := env == "production" || env == "prod" || env == "container"
	var encoder zapcore.Encoder
	if isContainer {
		encoder = zapcore.NewJSONEncoder(config)
	} else {
		encoder = zapcore.NewConsoleEncoder(config)
	}

	// 判断日志输出方式
	var writeSyncer zapcore.WriteSyncer
	logFile := os.Getenv("LOG_FILE")

	if isContainer && logFile == "" {
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		if logFile == "" {
			logFile = "app.log"
		}

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			writeSyncer = zapcore.AddSync(os.Stdout)
		} else {
			writeSyncer = zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(os.Stdout),
				zapcore.AddSync(file),
			)
		}
	}

	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		level,
	)

	// AddCaller添加调用者信息。去掉普通 Error 的 Stacktrace 避免输出一堆 github.com 的堆栈信息
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.DPanicLevel), // 仅在 Panic 时打印堆栈
	)

	logger = logger.With(zap.String("service", serviceName))

	return logger
}

// Ctx 从 context 中提取 OpenTelemetry 的 TraceID 和 SpanID，返回一个自动携带这些字段的子 Logger。
// 子 Logger 是轻量级的（只多了一个指针 + 几个字段），不会影响全局 Logger。
//
// 使用方式：
//
//	logger.Ctx(ctx, log).Info("创建用户成功", zap.String("user_id", uid))
//
// 输出效果：
//
//	{"level":"INFO", "message":"创建用户成功", "trace_id":"abc123...", "span_id":"def456...", "user_id":"u-001"}
func Ctx(ctx context.Context, log *zap.Logger) *zap.Logger {
	// 优先从 OTel Span 中获取（标准方式）
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return log.With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// 降级：从我们自定义的 context key 中取（兼容没有 OTel 的场景）
	traceID := trace.FromContext(ctx)
	if traceID != "" {
		return log.With(zap.String("trace_id", traceID))
	}

	return log
}
