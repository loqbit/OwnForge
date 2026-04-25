package logger

import (
	"context"
	"os"

	"github.com/loqbit/ownforge/pkg/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Create a new Logger instance
// serviceName identifies the service that produces the logs
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

	// Use Debug in development and Info in production
	level := zapcore.InfoLevel
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENV")
	}

	if env == "dev" || env == "development" {
		level = zapcore.DebugLevel
		// Use colored highlighting in development for easier reading
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Choose the encoder: JSON in container environments, Console locally
	isContainer := env == "production" || env == "prod" || env == "container"
	var encoder zapcore.Encoder
	if isContainer {
		encoder = zapcore.NewJSONEncoder(config)
	} else {
		encoder = zapcore.NewConsoleEncoder(config)
	}

	// Decide how logs should be written
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

	// AddCaller includes caller information. Ordinary Error stacktraces are disabled to avoid noisy github.com stacks.
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(0),
		zap.AddStacktrace(zapcore.DPanicLevel), // print stack traces only on Panic
	)

	logger = logger.With(zap.String("service", serviceName))

	return logger
}

// Ctx extracts the OpenTelemetry TraceID and SpanID from context and returns a child logger that carries them automatically.
// The child logger is lightweight and does not affect the global logger.
//
// Usage:
//
//	logger.Ctx(ctx, log).Info("create user successfully", zap.String("user_id", uid))
//
// Output:
//
//	{"level":"INFO", "message":"create user successfully", "trace_id":"abc123...", "span_id":"def456...", "user_id":"u-001"}
func Ctx(ctx context.Context, log *zap.Logger) *zap.Logger {
	// Prefer extracting from the OTel span first (the standard approach)
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return log.With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}

	// Fallback: read from our custom context key to support scenarios without OTel
	traceID := trace.FromContext(ctx)
	if traceID != "" {
		return log.With(zap.String("trace_id", traceID))
	}

	return log
}
