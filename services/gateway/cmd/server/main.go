package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ownforge/ownforge/pkg/health"
	"github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/pkg/otel"
	"github.com/ownforge/ownforge/pkg/probe"
	"github.com/ownforge/ownforge/pkg/ratelimiter"
	"github.com/ownforge/ownforge/pkg/redis"

	"github.com/ownforge/ownforge/services/gateway/internal/auth"
	"github.com/ownforge/ownforge/services/gateway/internal/config"
	"github.com/ownforge/ownforge/services/gateway/internal/grpcclient"
	"github.com/ownforge/ownforge/services/gateway/internal/gwproxy"
	"github.com/ownforge/ownforge/services/gateway/internal/handler"
	"github.com/ownforge/ownforge/services/gateway/internal/middleware"
	"github.com/ownforge/ownforge/services/gateway/internal/middleware/ratelimit"
	"github.com/ownforge/ownforge/services/gateway/internal/proxy"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 规范化配置
	cfg := config.LoadConfig()

	// 规范化日志
	log := logger.NewLogger("api-gateway")
	defer log.Sync()

	// 初始化 Redis
	redisClient := redis.Init(cfg.Redis, log)
	defer redisClient.Close()

	// 初始化 gRPC 客户端（使用纯 host:port 格式的 gRPC 地址）
	authClient, err := grpcclient.NewAuthClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("初始化 Auth gRPC 客户端失败", zap.Error(err))
	}
	userClient, err := grpcclient.NewUserClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("初始化 User gRPC 客户端失败", zap.Error(err))
	}

	// 初始化 Note gRPC 客户端（用于公开端点等不走 gRPC-Gateway 的场景）
	noteClientGrpc, err := grpcclient.NewNoteClient(cfg.Routes.GoNoteGRPC)
	if err != nil {
		log.Fatal("初始化 Note gRPC 客户端失败", zap.Error(err))
	}

	// 初始化 gRPC-Gateway 反向代理（自动代理所有 CRUD 路由）
	noteMux, err := gwproxy.NewNoteMux(context.Background(), cfg.Routes.GoNoteGRPC)
	if err != nil {
		log.Fatal("初始化 Note gRPC-Gateway 失败", zap.Error(err))
	}

	chatProxy := proxy.NewReverseProxy(cfg.Routes.GoChat)

	// 初始化 handler
	configHandler := handler.NewConfigHandler(cfg.Client)
	dashboardHandler := handler.NewDashboardHandler(userClient, noteClientGrpc, log)
	authHandler := handler.NewAuthHandler(authClient, cfg.SSOCookie, log)
	userHandler := handler.NewUserHandler(userClient, cfg.SSOCookie, log)
	publicNoteHandler := handler.NewPublicNoteHandler(noteClientGrpc, log)
	uploadHandler := handler.NewUploadHandler(noteClientGrpc, log)
	chatHandler := handler.NewChatHandler(chatProxy)

	// 初始化限流器
	BBRLimiter := ratelimiter.NewBBRLimiter(100, 10*time.Second, 80)
	IPLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)
	RouteLimiter := ratelimiter.NewTokenBucketLimiter(redisClient, log)
	UserLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)

	// 初始化鉴权依赖 (拿配置的 Secret 生成网关的 JWT 管理器)
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret)

	// 设置 Gin 运行模式：Release 模式关闭 GIN 自带的 debug 日志
	// 所有请求日志由我们的 zap 中间件统一管理
	if cfg.AppEnv == "production" || cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.ReleaseMode) // 即使开发环境也用 Release，避免 GIN debug 输出干扰 zap 日志
	}

	// [Bug 5 修复] 初始化 OpenTelemetry — 必须在注册 otelgin 中间件之前
	shutdown, err := otel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer shutdown(context.Background())

	r := gin.New()

	// 设置受信任的代理，消除 "You trusted all proxies" 警告
	// 仅信任内网代理（Docker 网络、K8s 集群内网、本机）
	r.SetTrustedProxies([]string{
		"127.0.0.1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})

	// [Bug 3 修复] 全局注入 logger 到每个请求的 context，确保所有路径都能记录日志
	r.Use(func(c *gin.Context) {
		c.Set("logger", log)
		c.Next()
	})

	// 探针端点：/healthz, /readyz, /metrics（注册在所有中间件之前）
	probe.Register(r, log,
		probe.WithRedis(redisClient),
	)

	// 下游依赖单独暴露，避免单个业务服务抖动时把整个网关摘出流量。
	dependencyChecker := health.NewChecker()
	dependencyChecker.AddCheck("go-note", newGRPCReadyCheck(cfg.Routes.GoNoteGRPC))
	dependencyChecker.AddCheck("go-chat", newHTTPReadyCheck(cfg.Routes.GoChat+"/readyz"))
	registerDependencyHealthRoute(r, dependencyChecker)

	// [CORS 防御层] — 从配置中读取白名单
	if len(cfg.Server.CorsOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.Server.CorsOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-Id", "X-Share-Password"},
			ExposeHeaders:    []string{"Content-Length", "X-Request-Id"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	// [全局前置拦截器]
	r.Use(otelgin.Middleware("api-gateway")) // 把每一条来到网关的请求打上 TraceID
	r.Use(logger.GinLogger(log))             // 记录响应时长和 TraceID（来自 common/logger）
	r.Use(logger.GinRecovery(log, true))     // 崩溃接管防闪退
	// 路由设计：
	api := r.Group("/api/v1")
	// Layer 1: IP 限流 (滑动窗口) - 所有请求都过
	api.Use(ratelimit.IPrateLimit(IPLimiter, 50, time.Second, log))
	// Layer 2: BBR 自适应限流 - 所有请求都过
	api.Use(ratelimit.BBRMiddleware(BBRLimiter, log))
	// 公共接口（无需 JWT 鉴权）
	api.GET("/config/client", configHandler.GetClientConfig)
	api.POST("/users/register", userHandler.Register)
	api.POST("/users/login", authHandler.Login)
	api.POST("/users/refresh", authHandler.RefreshToken)
	api.POST("/users/sso/exchange", authHandler.ExchangeSSO)
	api.POST("/users/phone/code", authHandler.SendPhoneCode)
	api.POST("/users/phone/entry", authHandler.PhoneAuthEntry)
	api.POST("/users/phone/password-login", authHandler.PhonePasswordLogin)
	// 公开笔记端点（无需鉴权）
	api.GET("/notes/public/snippets/:id", publicNoteHandler.GetPublic)
	api.GET("/notes/public/shares/:token", publicNoteHandler.GetPublicShare)

	{
		// 用户模块路由组（需 JWT 鉴权）
		usersGroup := api.Group("/users")
		// Layer 3: 路由限流 (令牌桶) - 单个服务级别
		usersGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		usersGroup.Use(middleware.JWTAuth(jwtManager, log))
		// Layer 4: 用户限流 (滑动窗口) - 登录用户过
		usersGroup.Use(ratelimit.UserRateLimit(UserLimiter, 20, time.Second, log))
		usersGroup.GET("/dashboard", dashboardHandler.GetDashboard)
		usersGroup.GET("/me/profile", userHandler.GetProfile)
		usersGroup.PUT("/me/profile", userHandler.UpdateProfile)
		usersGroup.POST("/password/change", userHandler.ChangePassword)
		usersGroup.POST("/password/set", userHandler.SetPassword)
		usersGroup.POST("/email/bind", userHandler.BindEmail)
		usersGroup.POST("/logout", authHandler.Logout)
		usersGroup.POST("/logout-all", userHandler.LogoutAllSessions)
	}
	{
		// 笔记模块路由组（需 JWT 鉴权）
		// gRPC-Gateway 自动代理所有 CRUD、分组、标签、模板路由
		notesGroup := api.Group("/notes")
		notesGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		notesGroup.Use(middleware.JWTAuth(jwtManager, log))
		notesGroup.Use(ratelimit.UserRateLimit(UserLimiter, 20, time.Second, log))

		// 文件上传保留为手写 handler（二进制流不适合 JSON gRPC-Gateway）
		notesGroup.POST("/uploads", uploadHandler.Upload)

		// gRPC-Gateway 透传路由。显式按方法注册，避免和手写上传路由冲突。
		noteGatewayHandler := gin.WrapH(gwproxy.WrapHandler(noteMux))

		notesGroup.GET("/me/snippets", noteGatewayHandler)
		notesGroup.GET("/me/snippets/recent", noteGatewayHandler)
		notesGroup.GET("/me/snippets/shared", noteGatewayHandler)
		notesGroup.GET("/me/snippets/favorites", noteGatewayHandler)
		notesGroup.GET("/snippets/search", noteGatewayHandler)
		notesGroup.GET("/snippets/:snippet_id", noteGatewayHandler)
		notesGroup.GET("/snippets/:snippet_id/ai", noteGatewayHandler)
		notesGroup.GET("/groups", noteGatewayHandler)
		notesGroup.GET("/groups/:group_id", noteGatewayHandler)
		notesGroup.GET("/tags", noteGatewayHandler)
		notesGroup.GET("/templates", noteGatewayHandler)
		notesGroup.GET("/templates/:template_id", noteGatewayHandler)

		notesGroup.POST("/snippets", noteGatewayHandler)
		notesGroup.POST("/snippets/from-template", noteGatewayHandler)
		notesGroup.POST("/snippets/from-share", noteGatewayHandler)
		notesGroup.POST("/snippets/:snippet_id/favorite", noteGatewayHandler)
		notesGroup.POST("/groups", noteGatewayHandler)
		notesGroup.POST("/tags", noteGatewayHandler)
		notesGroup.POST("/templates", noteGatewayHandler)
		notesGroup.POST("/uploads/presign", noteGatewayHandler)
		notesGroup.POST("/uploads/complete", noteGatewayHandler)
		notesGroup.POST("/shares", noteGatewayHandler)

		notesGroup.PUT("/snippets/:snippet_id", noteGatewayHandler)
		notesGroup.PUT("/snippets/:snippet_id/tags", noteGatewayHandler)
		notesGroup.PUT("/snippets/:snippet_id/move", noteGatewayHandler)
		notesGroup.PUT("/groups/:group_id", noteGatewayHandler)
		notesGroup.PUT("/tags/:tag_id", noteGatewayHandler)
		notesGroup.PUT("/templates/:template_id", noteGatewayHandler)

		notesGroup.DELETE("/snippets/:snippet_id", noteGatewayHandler)
		notesGroup.DELETE("/snippets/:snippet_id/favorite", noteGatewayHandler)
		notesGroup.DELETE("/groups/:group_id", noteGatewayHandler)
		notesGroup.DELETE("/tags/:tag_id", noteGatewayHandler)
		notesGroup.DELETE("/templates/:template_id", noteGatewayHandler)
		notesGroup.DELETE("/shares/:share_id", noteGatewayHandler)

		notesGroup.GET("/shares/my", noteGatewayHandler)
	}
	{
		chatGroup := api.Group("/chat")
		chatGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		chatGroup.Use(middleware.JWTAuth(jwtManager, log))
		chatGroup.Use(ratelimit.UserRateLimit(UserLimiter, 30, time.Second, log))
		chatGroup.Any("/*path", chatHandler.Proxy)
	}

	// 启动网关服务
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Info("API Gateway 启动成功",
			zap.String("port", cfg.Server.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("API网关运行异常: %v", zap.Error(err))
		}
	}()

	// 优雅停机,拦截系统的停机信号（如 Ctrl+C）
	quit := make(chan os.Signal, 1)
	// [Bug 4 修复] 同时监听 SIGINT(Ctrl+C) 和 SIGTERM(Docker/K8s 停止信号)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info("API Gateway 正在关闭...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("API网关强制关闭", zap.Error(err))
	}
	log.Info("API Gateway 已关闭")
}

func newHTTPReadyCheck(url string) health.CheckFunc {
	return func(ctx context.Context) error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("downstream readiness returned status %d", resp.StatusCode)
		}
		return nil
	}
}

func newGRPCReadyCheck(addr string) health.CheckFunc {
	return func(ctx context.Context) error {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer conn.Close()

		resp, err := healthgrpc.NewHealthClient(conn).Check(ctx, &healthgrpc.HealthCheckRequest{
			Service: "note.NoteService",
		})
		if err != nil {
			return err
		}
		if resp.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
			return fmt.Errorf("downstream grpc readiness returned status %s", resp.GetStatus().String())
		}
		return nil
	}
}

func registerDependencyHealthRoute(r *gin.Engine, checker *health.Checker) {
	r.GET("/healthz/deps", func(c *gin.Context) {
		allHealthy, results := checker.Evaluate(c.Request.Context())
		statusCode := http.StatusOK
		status := "healthy"
		if !allHealthy {
			statusCode = http.StatusServiceUnavailable
			status = "degraded"
		}

		c.JSON(statusCode, gin.H{
			"status": status,
			"checks": results,
		})
	})
}
