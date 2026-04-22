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
	// Normalize configuration
	cfg := config.LoadConfig()

	// Normalize logging
	log := logger.NewLogger("api-gateway")
	defer log.Sync()

	// Initialize Redis
	redisClient := redis.Init(cfg.Redis, log)
	defer redisClient.Close()

	// Initialize gRPC clients using plain host:port addresses
	authClient, err := grpcclient.NewAuthClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("failed to initialize Auth gRPC client", zap.Error(err))
	}
	userClient, err := grpcclient.NewUserClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("failed to initialize User gRPC client", zap.Error(err))
	}

	// Initialize the Note gRPC client for public endpoints and other flows that do not go through gRPC-Gateway
	noteClientGrpc, err := grpcclient.NewNoteClient(cfg.Routes.GoNoteGRPC)
	if err != nil {
		log.Fatal("failed to initialize Note gRPC client", zap.Error(err))
	}

	// Initialize the gRPC-Gateway reverse proxy for automatic CRUD routing
	noteMux, err := gwproxy.NewNoteMux(context.Background(), cfg.Routes.GoNoteGRPC)
	if err != nil {
		log.Fatal("failed to initialize Note gRPC-Gateway", zap.Error(err))
	}

	chatProxy := proxy.NewReverseProxy(cfg.Routes.GoChat)

	// Initialize handlers
	configHandler := handler.NewConfigHandler(cfg.Client)
	dashboardHandler := handler.NewDashboardHandler(userClient, noteClientGrpc, log)
	authHandler := handler.NewAuthHandler(authClient, cfg.SSOCookie, log)
	userHandler := handler.NewUserHandler(userClient, cfg.SSOCookie, log)
	publicNoteHandler := handler.NewPublicNoteHandler(noteClientGrpc, log)
	uploadHandler := handler.NewUploadHandler(noteClientGrpc, log)
	chatHandler := handler.NewChatHandler(chatProxy)

	// Initialize rate limiters
	BBRLimiter := ratelimiter.NewBBRLimiter(100, 10*time.Second, 80)
	IPLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)
	RouteLimiter := ratelimiter.NewTokenBucketLimiter(redisClient, log)
	UserLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)

	// Initialize auth dependencies using the configured secret to build the gateway JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret)

	// Set Gin mode: Release disables Gin's built-in debug logging
	// All request logs are handled uniformly by our zap middleware
	if cfg.AppEnv == "production" || cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.ReleaseMode) // Use Release even in development to avoid Gin debug output polluting zap logs
	}

	// [Bug 5 fix] Initialize OpenTelemetry before registering the otelgin middleware
	shutdown, err := otel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer shutdown(context.Background())

	r := gin.New()

	// Set trusted proxies to eliminate the "You trusted all proxies" warning
	// Trust only internal proxies (Docker network, K8s cluster network, localhost)
	r.SetTrustedProxies([]string{
		"127.0.0.1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})

	// [Bug 3 fix] Inject the logger into each request context globally so every code path can log
	r.Use(func(c *gin.Context) {
		c.Set("logger", log)
		c.Next()
	})

	// Probe endpoints: /healthz, /readyz, /metrics (registered before all middleware)
	probe.Register(r, log,
		probe.WithRedis(redisClient),
	)

	// Expose downstream dependency health separately so one unstable service does not remove the entire gateway from traffic.
	dependencyChecker := health.NewChecker()
	dependencyChecker.AddCheck("go-note", newGRPCReadyCheck(cfg.Routes.GoNoteGRPC))
	dependencyChecker.AddCheck("go-chat", newHTTPReadyCheck(cfg.Routes.GoChat+"/readyz"))
	registerDependencyHealthRoute(r, dependencyChecker)

	// [CORS defense layer] read the allowlist from config
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

	// [Global pre-processing interceptors]
	r.Use(otelgin.Middleware("api-gateway")) // Attach a TraceID to every request reaching the gateway
	r.Use(logger.GinLogger(log))             // Record response time and TraceID (from common/logger)
	r.Use(logger.GinRecovery(log, true))     // Recover from panics to prevent crashes
	// Routing design:
	api := r.Group("/api/v1")
	// Layer 1: IP rate limiting (sliding window) - applies to all requests
	api.Use(ratelimit.IPrateLimit(IPLimiter, 50, time.Second, log))
	// Layer 2: BBR adaptive rate limiting - applies to all requests
	api.Use(ratelimit.BBRMiddleware(BBRLimiter, log))
	// Public endpoints (no JWT authentication required)
	api.GET("/config/client", configHandler.GetClientConfig)
	api.POST("/users/register", userHandler.Register)
	api.POST("/users/login", authHandler.Login)
	api.POST("/users/refresh", authHandler.RefreshToken)
	api.POST("/users/sso/exchange", authHandler.ExchangeSSO)
	api.POST("/users/phone/code", authHandler.SendPhoneCode)
	api.POST("/users/phone/entry", authHandler.PhoneAuthEntry)
	api.POST("/users/phone/password-login", authHandler.PhonePasswordLogin)
	// Public note endpoints (no authentication required)
	api.GET("/notes/public/snippets/:id", publicNoteHandler.GetPublic)
	api.GET("/notes/public/shares/:token", publicNoteHandler.GetPublicShare)

	{
		// User route group (JWT authentication required)
		usersGroup := api.Group("/users")
		// Layer 3: Route-level rate limiting (token bucket) - per service
		usersGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		usersGroup.Use(middleware.JWTAuth(jwtManager, log))
		// Layer 4: User-level rate limiting (sliding window) - for authenticated users
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
		// Note route group (JWT authentication required)
		// gRPC-Gateway automatically proxies CRUD, group, tag, and template routes
		notesGroup := api.Group("/notes")
		notesGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		notesGroup.Use(middleware.JWTAuth(jwtManager, log))
		notesGroup.Use(ratelimit.UserRateLimit(UserLimiter, 20, time.Second, log))

		// Keep file uploads in a handwritten handler because binary streams do not fit JSON gRPC-Gateway well
		notesGroup.POST("/uploads", uploadHandler.Upload)

		// Pass-through routes for gRPC-Gateway. Register them explicitly by method to avoid conflicts with the handwritten upload route.
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

	// Start the gateway service
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Info("API Gateway started successfully",
			zap.String("port", cfg.Server.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("API Gateway runtime error: %v", zap.Error(err))
		}
	}()

	// Graceful shutdown: intercept system shutdown signals such as Ctrl+C
	quit := make(chan os.Signal, 1)
	// [Bug 4 fix] Listen to both SIGINT (Ctrl+C) and SIGTERM (Docker/K8s stop signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info("API Gateway is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("API Gateway forced shutdown", zap.Error(err))
	}
	log.Info("API Gateway closed")
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
