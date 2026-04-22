package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/ownforge/ownforge/pkg/logger"
	commonOtel "github.com/ownforge/ownforge/pkg/otel"
	"github.com/ownforge/ownforge/pkg/probe"
	commonRedis "github.com/ownforge/ownforge/pkg/redis"
	"github.com/ownforge/ownforge/services/user-platform/internal/appcontainer"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/bootstrap"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/database"
	platformidgen "github.com/ownforge/ownforge/services/user-platform/internal/platform/idgen"
	httprouter "github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/router"
	"go.uber.org/zap"
)

// @title           User Platform Service
// @version         1.0
// @description     User center service providing registration and login APIs
// @host            localhost:8081
// @BasePath        /api/v1
func main() {
	log := logger.NewLogger("user-http")
	defer log.Sync()

	cfg := config.LoadConfig()

	// 1. Initialize the underlying infrastructure.
	entClient, redisClient, idgenClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()
	defer idgenClient.Close()

	// 2. Initialize OpenTelemetry tracing.
	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	// 3. Wire dependencies and assemble components.
	router := buildRouter(cfg, entClient, redisClient, idgenClient, log)

	// 4. Run and handle graceful shutdown.
	runServer(router, cfg.Server.Port, log)
}

// initInfra initializes the infrastructure.
func initInfra(cfg *config.Config, log *zap.Logger) (*ent.Client, *redis.Client, platformidgen.Client) {
	idgenClient, err := platformidgen.New(cfg.IDGenerator.Addr)
	if err != nil {
		log.Fatal("failed to initialize ID generator client", zap.Error(err))
	}
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	if err := bootstrap.EnsureDefaultApps(context.Background(), entClient, log, bootstrap.DefaultApps); err != nil {
		log.Fatal("failed to initialize default apps", zap.Error(err))
	}
	redisClient := commonRedis.Init(cfg.Redis, log)

	return entClient, redisClient, idgenClient
}

// buildRouter wires dependencies into the router.
func buildRouter(cfg *config.Config, entClient *ent.Client, redisClient *redis.Client, idgenClient platformidgen.Client, log *zap.Logger) *gin.Engine {
	container := appcontainer.Build(cfg, entClient, redisClient, idgenClient, log)
	r := gin.New()

	// Probe endpoints: /healthz, /readyz, and /metrics, registered before business middleware.
	probe.Register(r, log,
		probe.WithCheck("postgres", func(ctx context.Context) error {
			_, err := entClient.User.Query().Exist(ctx)
			return err
		}),
		probe.WithRedis(redisClient),
	)

	httprouter.SetupRouter(httprouter.Dependencies{
		Engine:      r,
		UserHandler: container.UserHandler,
		JWTManager:  container.JWTManager,
		Logger:      log,
	})

	return r
}

// runServer starts the HTTP server and shuts it down gracefully after receiving a stop signal.
func runServer(router *gin.Engine, port string, log *zap.Logger) {
	// Start the HTTP server.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
	go func() {
		log.Info("HTTP server started", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed to listen", zap.Error(err))
		}
	}()

	// Wait for a shutdown signal.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("received shutdown signal, starting graceful shutdown...")

	// Gracefully shut down the HTTP server and wait for in-flight requests to finish.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("HTTP server forced to exit", zap.Error(err))
	}

	log.Info("all services exited safely")
}
