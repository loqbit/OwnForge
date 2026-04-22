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
	"github.com/ownforge/ownforge/pkg/rpc"
	"github.com/ownforge/ownforge/services/user-platform/internal/appcontainer"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/bootstrap"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/database"
	transportgrpc "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc"

	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("user-grpc")
	defer log.Sync()

	cfg := config.LoadConfig()

	// 初始化底层基础设施
	entClient, redisClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()

	// 初始化 OpenTelemetry 链路追踪
	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	// 探针：独立管理端口 + gRPC Health 同步
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

	// 依赖注入与组件装配
	container := buildContainer(cfg, entClient, redisClient, log)
	grpcServer := transportgrpc.SetupServer(transportgrpc.ServerDependencies{
		UserService:    container.UserService,
		ProfileService: container.ProfileService,
		AuthService:    container.AuthService,
		HealthServer:   grpcHealthServer,
		Logger:         log,
	})

	// 阻塞运行与优雅停机
	runServer(grpcServer, grpcHealthServer, cfg.GRPCServer.Port, log)
}

// initInfra 初始化基础设施
func initInfra(cfg *config.Config, log *zap.Logger) (*ent.Client, *redis.Client) {
	if err := rpc.InitIDGenClient(cfg.IDGenerator.Addr); err != nil {
		log.Fatal("初始化 ID 生成器客户端失败", zap.Error(err))
	}

	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	if err := bootstrap.EnsureDefaultApps(context.Background(), entClient, log, bootstrap.DefaultApps); err != nil {
		log.Fatal("初始化默认应用失败", zap.Error(err))
	}
	redisClient := commonRedis.Init(cfg.Redis, log)

	return entClient, redisClient
}

// buildContainer 构建应用运行容器。
func buildContainer(cfg *config.Config, entClient *ent.Client, redisClient *redis.Client, log *zap.Logger) *appcontainer.Container {
	return appcontainer.Build(cfg, entClient, redisClient, log)
}

// runServer 启动 gRPC 服务器，监听停机信号后优雅退出
func runServer(s *grpc.Server, healthServer *grpchealth.Server, port string, log *zap.Logger) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("gRPC 端口监听失败", zap.Error(err))
	}

	go func() {
		log.Info("gRPC 服务已启动", zap.String("port", port))
		if err := s.Serve(lis); err != nil {
			log.Fatal("gRPC 服务异常终止", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("收到停机信号，开始优雅退出...")
	probe.GRPCShutdown(healthServer, "user.UserService", "user.AuthService")
	s.GracefulStop()

	log.Info("gRPC 服务已安全退出")
}
