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

	"github.com/luckysxx/common/logger"
	commonOtel "github.com/luckysxx/common/otel"
	"github.com/luckysxx/common/probe"
	commonRedis "github.com/luckysxx/common/redis"
	"github.com/luckysxx/common/rpc"
	"github.com/ownforge/ownforge/services/user-platform/internal/appcontainer"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/bootstrap"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/database"
	httprouter "github.com/ownforge/ownforge/services/user-platform/internal/transport/http/server/router"
	"go.uber.org/zap"
)

// @title           User Platform Service
// @version         1.0
// @description     用户中心服务，提供注册、登录功能
// @host            localhost:8081
// @BasePath        /api/v1
func main() {
	log := logger.NewLogger("user-http")
	defer log.Sync()

	cfg := config.LoadConfig()

	// 1. 初始化底层基础设施
	entClient, redisClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()

	// 2. 初始化 OpenTelemetry 链路追踪
	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	// 3. 依赖注入与组件装配
	router := buildRouter(cfg, entClient, redisClient, log)

	// 4. 阻塞运行与优雅停机
	runServer(router, cfg.Server.Port, log)
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

// buildRouter 依赖注入装配
func buildRouter(cfg *config.Config, entClient *ent.Client, redisClient *redis.Client, log *zap.Logger) *gin.Engine {
	container := appcontainer.Build(cfg, entClient, redisClient, log)
	r := gin.New()

	// 探针端点：/healthz, /readyz, /metrics（注册在业务中间件之前）
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

// runServer 启动 HTTP 服务器，监听停机信号后优雅退出
func runServer(router *gin.Engine, port string, log *zap.Logger) {
	// 启动 HTTP 服务器
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
	go func() {
		log.Info("HTTP 服务已启动", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP 服务监听失败", zap.Error(err))
		}
	}()

	// 监听停机信号
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("收到停机信号，开始优雅退出...")

	// 优雅关闭 HTTP 服务器（等待正在处理的请求完成）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("HTTP 服务强制退出", zap.Error(err))
	}

	log.Info("所有服务已安全退出")
}
