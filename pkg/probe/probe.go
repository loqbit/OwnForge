// Package probe provides unified registration for operational probe endpoints.
//
// It wraps /healthz (liveness), /readyz (readiness), and /metrics (Prometheus metrics),
// so each microservice can register all operational endpoints with a single line of code and avoid boilerplate.
//
// It supports two usage modes:
//
// Mode 1: mount on an existing Gin engine (for HTTP services):
//
//	probe.Register(r, log,
//	    probe.WithRedis(redisClient),
//	    probe.WithEntDB("postgres", entClient),
//	)
//
// Mode 2: start a dedicated admin port (for gRPC or pure consumer services):
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
	"github.com/loqbit/ownforge/pkg/health"
	"github.com/loqbit/ownforge/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	grpchealth "google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

// Pinger is an interface that performs connection liveness checks.
// *sql.DB satisfies this interface directly.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Option is a functional option for probe configuration.
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

// WithCheck registers a custom named health check.
func WithCheck(name string, fn health.CheckFunc) Option {
	return func(c *probeConfig) {
		c.checks[name] = fn
	}
}

// WithRedis registers a Redis liveness check.
func WithRedis(rdb *redis.Client) Option {
	return func(c *probeConfig) {
		if rdb != nil {
			c.checks["redis"] = func(ctx context.Context) error {
				return rdb.Ping(ctx).Err()
			}
		}
	}
}

// WithPinger registers a database connection check.
// name identifies the database instance (for example, "postgres"), and pinger can be any object that implements PingContext (such as *sql.DB).
func WithPinger(name string, p Pinger) Option {
	return func(c *probeConfig) {
		if p != nil {
			c.checks[name] = func(ctx context.Context) error {
				return p.PingContext(ctx)
			}
		}
	}
}

// WithGRPCHealth syncs health-check results to the native gRPC Health service.
// services are the gRPC service names to register (for example, "note.NoteService").
func WithGRPCHealth(srv *grpchealth.Server, services ...string) Option {
	return func(c *probeConfig) {
		c.grpcHealth = srv
		c.grpcServices = services
	}
}

// WithoutMetrics disables /metrics endpoint registration (enabled by default).
func WithoutMetrics() Option {
	return func(c *probeConfig) {
		c.enableMetrics = false
	}
}

// ── Mode 1: Register - mount on a Gin engine ─────────────────────────────

// Register adds /healthz, /readyz, and /metrics to an existing Gin engine.
// Call it before all business middleware to avoid rate limiting or auth interception.
//
// Example:
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

	log.Info("probe endpoints registered on Gin",
		zap.Int("checks", len(cfg.checks)),
		zap.Bool("metrics", cfg.enableMetrics),
	)
}

// ── Mode 2: Serve - dedicated admin port ────────────────────────────────────

// Serve starts a dedicated HTTP admin port exposing /healthz, /readyz, and /metrics.
// This is suitable for pure gRPC services, Kafka consumers, and other processes without an HTTP engine.
//
// It returns a shutdown function that callers should invoke on exit to gracefully close the admin port.
// The admin server also shuts down automatically when ctx is canceled.
//
// Example:
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

	// If gRPC Health is configured, start a background sync goroutine.
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
		log.Info("probe admin port started",
			zap.String("addr", addr),
			zap.Int("checks", len(cfg.checks)),
			zap.Strings("endpoints", []string{"/healthz", "/readyz", "/metrics"}),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("probe admin port error", zap.Error(err))
		}
	}()

	// Watch ctx cancellation and close automatically.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Return a manual shutdown function.
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shut down probe admin port", zap.Error(err))
		}
	}
}

// GRPCShutdown sets all registered gRPC Health services to NOT_SERVING.
// Call it during graceful gRPC shutdown.
func GRPCShutdown(srv *grpchealth.Server, services ...string) {
	srv.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	for _, svc := range services {
		srv.SetServingStatus(svc, healthgrpc.HealthCheckResponse_NOT_SERVING)
	}
}

// ── Internal implementation ──────────────────────────────────────────────────────

// startHealthSync periodically syncs common/health results to the native gRPC Health service.
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
			log.Debug("gRPC health status updated", zap.String("status", status.String()))
			return
		}

		log.Warn("gRPC health status updated",
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
