package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	grpchealth "google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/loqbit/ownforge/pkg/health"
	"github.com/loqbit/ownforge/pkg/logger"
	commonOtel "github.com/loqbit/ownforge/pkg/otel"
	"github.com/loqbit/ownforge/pkg/probe"
	commonRedis "github.com/loqbit/ownforge/pkg/redis"
	"github.com/loqbit/ownforge/services/user-platform/internal/appcontainer"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/bootstrap"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/config"
	"github.com/loqbit/ownforge/services/user-platform/internal/platform/database"
	platformidgen "github.com/loqbit/ownforge/services/user-platform/internal/platform/idgen"
	transportgrpc "github.com/loqbit/ownforge/services/user-platform/internal/transport/grpc"
	httprouter "github.com/loqbit/ownforge/services/user-platform/internal/transport/http/server/router"
)

func main() {
	_ = godotenv.Load()

	log := logger.NewLogger("user-platform")
	defer log.Sync()

	cfg := config.LoadConfig()

	entClient, redisClient, idgenClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()
	defer idgenClient.Close()

	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	container := appcontainer.Build(cfg, entClient, redisClient, idgenClient, log)

	grpcHealthServer := grpchealth.NewServer()
	checker := buildHealthChecker(entClient, redisClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startGRPCHealthSync(ctx, checker, grpcHealthServer, "user.UserService", "user.AuthService")

	grpcServer := transportgrpc.SetupServer(transportgrpc.ServerDependencies{
		UserService:    container.UserService,
		ProfileService: container.ProfileService,
		AuthService:    container.AuthService,
		HealthServer:   grpcHealthServer,
		Logger:         log,
	})

	grpcAddr := ":" + cfg.GRPCServer.Port
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen on gRPC port", zap.Error(err))
	}

	go func() {
		log.Info("gRPC server started", zap.String("port", cfg.GRPCServer.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC server terminated unexpectedly", zap.Error(err))
		}
	}()

	r := gin.New()
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

	httpServer := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Info("HTTP server started", zap.String("port", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed to listen", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("received shutdown signal, starting graceful shutdown...")
	cancel()
	probe.GRPCShutdown(grpcHealthServer, "user.UserService", "user.AuthService")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shut down HTTP server", zap.Error(err))
	}

	grpcServer.GracefulStop()
	log.Info("user-platform single-process server exited safely")
}

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

func buildHealthChecker(entClient *ent.Client, redisClient *redis.Client) *health.Checker {
	checker := health.NewChecker()
	checker.AddCheck("postgres", func(ctx context.Context) error {
		_, err := entClient.User.Query().Exist(ctx)
		return err
	})
	checker.AddCheck("redis", func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
	return checker
}

func startGRPCHealthSync(ctx context.Context, checker *health.Checker, srv *grpchealth.Server, services ...string) {
	update := func() {
		allHealthy, _ := checker.Evaluate(context.Background())
		status := healthgrpc.HealthCheckResponse_SERVING
		if !allHealthy {
			status = healthgrpc.HealthCheckResponse_NOT_SERVING
		}

		srv.SetServingStatus("", status)
		for _, service := range services {
			srv.SetServingStatus(service, status)
		}
	}

	update()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				update()
			}
		}
	}()
}
