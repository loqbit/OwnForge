package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"

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
	transportgrpc "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc"

	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("user-grpc")
	defer log.Sync()

	cfg := config.LoadConfig()

	// Initialize the underlying infrastructure.
	entClient, redisClient, idgenClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()
	defer idgenClient.Close()

	// Initialize OpenTelemetry tracing.
	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	// Probes: dedicated admin port plus synchronized gRPC health reporting.
	grpcHealthServer := grpchealth.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	probeShutdown := probe.Serve(ctx, ":"+cfg.Metrics.Port, log,
		probe.WithCheck("postgres", func(ctx context.Context) error {
			_, err := entClient.User.Query().Exist(ctx)
			return err
		}),
		probe.WithRedis(redisClient),
		probe.WithGRPCHealth(grpcHealthServer, "user.UserService", "user.AuthService"),
	)
	defer probeShutdown()

	// Wire dependencies and assemble components.
	container := buildContainer(cfg, entClient, redisClient, idgenClient, log)
	grpcServer := transportgrpc.SetupServer(transportgrpc.ServerDependencies{
		UserService:    container.UserService,
		ProfileService: container.ProfileService,
		AuthService:    container.AuthService,
		HealthServer:   grpcHealthServer,
		Logger:         log,
	})

	// Run and handle graceful shutdown.
	runServer(grpcServer, grpcHealthServer, cfg.GRPCServer.Port, log)
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

// buildContainer builds the application runtime container.
func buildContainer(cfg *config.Config, entClient *ent.Client, redisClient *redis.Client, idgenClient platformidgen.Client, log *zap.Logger) *appcontainer.Container {
	return appcontainer.Build(cfg, entClient, redisClient, idgenClient, log)
}

// runServer starts the gRPC server and shuts it down gracefully after receiving a stop signal.
func runServer(s *grpc.Server, healthServer *grpchealth.Server, port string, log *zap.Logger) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("failed to listen on gRPC port", zap.Error(err))
	}

	go func() {
		log.Info("gRPC server started", zap.String("port", port))
		if err := s.Serve(lis); err != nil {
			log.Fatal("gRPC server terminated unexpectedly", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("received shutdown signal, starting graceful shutdown...")
	probe.GRPCShutdown(healthServer, "user.UserService", "user.AuthService")
	s.GracefulStop()

	log.Info("gRPC server exited safely")
}
