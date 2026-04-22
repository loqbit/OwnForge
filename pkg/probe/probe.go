// Package probe 提供统一的运维探针端点注册能力。
//
// 封装了 /healthz（存活探针）、/readyz（就绪探针）和 /metrics（Prometheus 指标）,
// 使每个微服务只需一行代码即可完成全部运维端点的注册，消除重复模板代码。
//
// 提供两种使用模式：
//
// 模式 1 — 挂载到已有 Gin 引擎（适用于 HTTP 服务）：
//
//	probe.Register(r, log,
//	    probe.WithRedis(redisClient),
//	    probe.WithEntDB("postgres", entClient),
//	)
//
// 模式 2 — 启动独立管理端口（适用于 gRPC / 纯消费者服务）：
//
//	shutdown := probe.Serve(ctx, ":9094", log,
//	    probe.WithRedis(redisClient),
//	    probe.WithGRPCHealth(grpcHealthServer, "note.NoteService"),
//	)
//	defer shutdown()
package probe

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ownforge/ownforge/pkg/health"
	"github.com/ownforge/ownforge/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	grpchealth "google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

// Pinger 是一个能执行连接存活探测的接口。
// *sql.DB 直接满足此接口。
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Option 是配置探针的函数选项。
type Option func(*probeConfig)

type probeConfig struct {
	checks        map[string]health.CheckFunc
	grpcHealth    *grpchealth.Server
	grpcServices  []string
	enableMetrics bool
}

func defaultConfig() *probeConfig {
	return &probeConfig{
		checks:        make(map[string]health.CheckFunc),
		enableMetrics: true,
	}
}

// ── Functional Options ──────────────────────────────────────────────

// WithCheck 注册一个自定义的命名健康检查项。
func WithCheck(name string, fn health.CheckFunc) Option {
	return func(c *probeConfig) {
		c.checks[name] = fn
	}
}

// WithRedis 注册 Redis 连接存活检查。
func WithRedis(rdb *redis.Client) Option {
	return func(c *probeConfig) {
		if rdb != nil {
			c.checks["redis"] = func(ctx context.Context) error {
				return rdb.Ping(ctx).Err()
			}
		}
	}
}

// WithPinger 注册数据库连接检查。
// name 用于标识数据库实例（如 "postgres"），pinger 是任何实现了 PingContext 的对象（*sql.DB 等）。
func WithPinger(name string, p Pinger) Option {
	return func(c *probeConfig) {
		if p != nil {
			c.checks[name] = func(ctx context.Context) error {
				return p.PingContext(ctx)
			}
		}
	}
}

// WithGRPCHealth 将健康检查结果同步到 gRPC 原生 Health 服务。
// services 是需要注册的 gRPC 服务名（如 "note.NoteService"）。
func WithGRPCHealth(srv *grpchealth.Server, services ...string) Option {
	return func(c *probeConfig) {
		c.grpcHealth = srv
		c.grpcServices = services
	}
}

// WithoutMetrics 禁用 /metrics 端点注册（默认开启）。
func WithoutMetrics() Option {
	return func(c *probeConfig) {
		c.enableMetrics = false
	}
}

// ── 模式 1：Register — 挂载到 Gin 引擎 ─────────────────────────────

// Register 将 /healthz、/readyz 和 /metrics 注册到已有的 Gin 引擎上。
// 应在所有业务中间件之前调用，避免被限流或鉴权拦截。
//
// 使用示例：
//
//	r := gin.New()
//	probe.Register(r, log,
//	    probe.WithRedis(redisClient),
//	    probe.WithEntDB("postgres", entClient),
//	)
func Register(r *gin.Engine, log *zap.Logger, opts ...Option) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	checker := health.NewChecker()
	for name, fn := range cfg.checks {
		checker.AddCheck(name, fn)
	}
	checker.Register(r)

	if cfg.enableMetrics {
		r.GET("/metrics", metrics.GinMetricsHandler())
		r.Use(metrics.GinMetrics())
	}

	log.Info("probe 端点已注册到 Gin",
		zap.Int("checks", len(cfg.checks)),
		zap.Bool("metrics", cfg.enableMetrics),
	)
}

// ── 模式 2：Serve — 独立管理端口 ────────────────────────────────────

// Serve 启动一个独立的 HTTP 管理端口，暴露 /healthz、/readyz 和 /metrics。
// 适用于纯 gRPC 服务、Kafka 消费者等没有 HTTP 引擎的进程。
//
// 返回一个 shutdown 函数，调用者应在退出时调用以优雅关闭管理端口。
// ctx 被取消时管理服务器也会自动关闭。
//
// 使用示例：
//
//	shutdown := probe.Serve(ctx, ":9094", log,
//	    probe.WithRedis(redisClient),
//	    probe.WithEntDB("postgres", entClient),
//	    probe.WithGRPCHealth(grpcHealthServer, "note.NoteService"),
//	)
//	defer shutdown()
func Serve(ctx context.Context, addr string, log *zap.Logger, opts ...Option) func() {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	checker := health.NewChecker()
	for name, fn := range cfg.checks {
		checker.AddCheck(name, fn)
	}

	// 如果配置了 gRPC Health，启动后台同步协程
	if cfg.grpcHealth != nil {
		startHealthSync(checker, cfg.grpcHealth, cfg.grpcServices, log)
	}

	mux := http.NewServeMux()
	checker.RegisterHTTP(mux)
	if cfg.enableMetrics {
		mux.Handle("/metrics", promhttp.Handler())
	}

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Info("probe 管理端口已启动",
			zap.String("addr", addr),
			zap.Int("checks", len(cfg.checks)),
			zap.Strings("endpoints", []string{"/healthz", "/readyz", "/metrics"}),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("probe 管理端口异常", zap.Error(err))
		}
	}()

	// 监听 ctx 取消，自动关闭
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// 返回手动 shutdown 函数
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("probe 管理端口关闭失败", zap.Error(err))
		}
	}
}

// GRPCShutdown 设置所有已注册的 gRPC Health 服务为 NOT_SERVING。
// 应在 gRPC 优雅停机时调用。
func GRPCShutdown(srv *grpchealth.Server, services ...string) {
	srv.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	for _, svc := range services {
		srv.SetServingStatus(svc, healthgrpc.HealthCheckResponse_NOT_SERVING)
	}
}

// ── 内部实现 ──────────────────────────────────────────────────────

// startHealthSync 将 common/health 的检查结果周期性同步到 gRPC 原生 Health 服务。
func startHealthSync(checker *health.Checker, srv *grpchealth.Server, services []string, log *zap.Logger) {
	var lastStatus healthgrpc.HealthCheckResponse_ServingStatus
	var initialized bool

	update := func() {
		allHealthy, results := checker.Evaluate(context.Background())
		status := healthgrpc.HealthCheckResponse_SERVING
		if !allHealthy {
			status = healthgrpc.HealthCheckResponse_NOT_SERVING
		}

		srv.SetServingStatus("", status)
		for _, svc := range services {
			srv.SetServingStatus(svc, status)
		}

		if initialized && status == lastStatus {
			return
		}
		lastStatus = status
		initialized = true

		if allHealthy {
			log.Debug("gRPC health 状态已更新", zap.String("status", status.String()))
			return
		}

		log.Warn("gRPC health 状态已更新",
			zap.String("status", status.String()),
			zap.Any("checks", results),
		)
	}

	update()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			update()
		}
	}()
}
