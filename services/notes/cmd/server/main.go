package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/joho/godotenv"
	"github.com/luckysxx/common/health"
	"github.com/luckysxx/common/logger"
	commonOtel "github.com/luckysxx/common/otel"
	"github.com/luckysxx/common/probe"
	notepb "github.com/luckysxx/common/proto/note"
	commonRedis "github.com/luckysxx/common/redis"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/event"
	"github.com/ownforge/ownforge/services/notes/internal/platform/config"
	"github.com/ownforge/ownforge/services/notes/internal/platform/database"
	platformidgen "github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	"github.com/ownforge/ownforge/services/notes/internal/platform/storage"
	aimetadatarepo "github.com/ownforge/ownforge/services/notes/internal/repository/aimetadata"
	groupsvc "github.com/ownforge/ownforge/services/notes/internal/service/group"
	lineagesvc "github.com/ownforge/ownforge/services/notes/internal/service/lineage"
	sharesvc "github.com/ownforge/ownforge/services/notes/internal/service/share"
	snippetsvc "github.com/ownforge/ownforge/services/notes/internal/service/snippet"
	tagsvc "github.com/ownforge/ownforge/services/notes/internal/service/tag"
	templatesvc "github.com/ownforge/ownforge/services/notes/internal/service/template"
	uploadsvc "github.com/ownforge/ownforge/services/notes/internal/service/upload"
	aimetadatastore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/aimetadata"
	groupstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/group"
	lineagestore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/lineage"
	sharestore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/share"
	snippetstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/snippet"
	tagstore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/tag"
	templatestore "github.com/ownforge/ownforge/services/notes/internal/store/entstore/template"
	transportgrpc "github.com/ownforge/ownforge/services/notes/internal/transport/grpc"
	httpgwproxy "github.com/ownforge/ownforge/services/notes/internal/transport/http/server/gwproxy"
	"github.com/ownforge/ownforge/services/notes/internal/transport/http/server/handler"
	httprouter "github.com/ownforge/ownforge/services/notes/internal/transport/http/server/router"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpchealth "google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

type services struct {
	snippetSvc     snippetsvc.SnippetService
	groupSvc       groupsvc.GroupService
	tagSvc         tagsvc.TagService
	templateSvc    templatesvc.TemplateService
	lineageSvc     lineagesvc.Service
	shareSvc       sharesvc.Service
	uploadSvc      uploadsvc.UploadService
	aimetadataRepo aimetadatarepo.Repository
}

type handlers struct {
	groupHandler  *handler.GroupHandler
	shareHandler  *handler.ShareHandler
	uploadHandler *handler.UploadHandler
}

func main() {
	_ = godotenv.Load()

	log := logger.NewLogger("go-note")
	defer log.Sync()

	cfg := config.LoadConfig()

	entClient, redisClient, idgenClient := initInfra(cfg, log)
	defer entClient.Close()
	defer redisClient.Close()
	defer idgenClient.Close()

	otelShutdown, err := commonOtel.InitTracer(cfg.OTel)
	if err != nil {
		log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	svcs := buildServices(cfg, redisClient, entClient, idgenClient, log)
	hs := buildHandlers(svcs, log)

	if err := svcs.templateSvc.SeedSystemTemplates(context.Background()); err != nil {
		log.Error("seed 系统模板失败", zap.Error(err))
	}

	grpcHealthServer := grpchealth.NewServer()
	checker := buildHealthChecker(entClient, redisClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startGRPCHealthSync(ctx, checker, grpcHealthServer, "note.NoteService")

	grpcServer := transportgrpc.SetupServer(
		svcs.snippetSvc,
		svcs.groupSvc,
		svcs.tagSvc,
		svcs.templateSvc,
		svcs.lineageSvc,
		svcs.shareSvc,
		svcs.uploadSvc,
		svcs.aimetadataRepo,
		grpcHealthServer,
		log,
	)

	grpcAddr := ":" + cfg.GRPCServer.Port
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("gRPC 端口监听失败", zap.Error(err))
	}

	go func() {
		log.Info("gRPC 服务已启动", zap.String("port", cfg.GRPCServer.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC 服务异常终止", zap.Error(err))
		}
	}()

	gwMux := runtime.NewServeMux(
		runtime.WithMetadata(func(_ context.Context, r *http.Request) metadata.MD {
			md := metadata.MD{}
			if uid := r.Header.Get("X-User-Id"); uid != "" {
				md.Set("x-user-id", uid)
			}
			return md
		}),
	)

	gwAddr := "127.0.0.1:" + cfg.GRPCServer.Port
	if err := notepb.RegisterNoteServiceHandlerFromEndpoint(
		context.Background(),
		gwMux,
		gwAddr,
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	); err != nil {
		log.Fatal("注册 grpc-gateway 失败", zap.Error(err))
	}

	r := gin.New()
	probe.Register(r, log,
		probe.WithCheck("postgres", func(ctx context.Context) error {
			_, err := entClient.Snippet.Query().Limit(1).Count(ctx)
			return err
		}),
		probe.WithRedis(redisClient),
	)

	httprouter.SetupRouter(r, hs.uploadHandler, hs.shareHandler, hs.groupHandler, log)

	gatewayHandler := httpgwproxy.WrapHandler(gwMux)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/notes") {
			gatewayHandler.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
			"msg":  "not found",
			"data": nil,
		})
	})

	httpServer := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Info("HTTP 服务已启动", zap.String("port", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP 服务监听失败", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("收到停机信号，开始优雅退出...")
	cancel()
	probe.GRPCShutdown(grpcHealthServer, "note.NoteService")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP 服务关闭失败", zap.Error(err))
	}

	grpcServer.GracefulStop()
	log.Info("go-note 单主进程已安全退出")
}

func initInfra(cfg *config.Config, log *zap.Logger) (*ent.Client, *redis.Client, platformidgen.Client) {
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	redisClient := commonRedis.Init(cfg.Redis, log)
	idgenClient, err := platformidgen.New(cfg.IDGenerator.Addr)
	if err != nil {
		log.Fatal("初始化 id-generator 客户端失败", zap.Error(err))
	}

	return entClient, redisClient, idgenClient
}

func buildServices(cfg *config.Config, redisClient *redis.Client, entClient *ent.Client, idgenClient platformidgen.Client, log *zap.Logger) services {
	snippetRepo := snippetstore.New(entClient)
	aimetadataRepo := aimetadatastore.New(entClient)
	groupRepo := groupstore.New(entClient)
	tagRepo := tagstore.New(entClient)
	shareRepo := sharestore.New(entClient)
	templateRepo := templatestore.New(entClient)
	lineageRepo := lineagestore.New(entClient)

	minioStorage, err := storage.NewMinIOStorage(storage.MinIOConfig{
		Endpoint:       cfg.MinIO.Endpoint,
		PublicEndpoint: cfg.MinIO.PublicEndpoint,
		AccessKey:      cfg.MinIO.AccessKey,
		SecretKey:      cfg.MinIO.SecretKey,
		Bucket:         cfg.MinIO.Bucket,
		UseSSL:         cfg.MinIO.UseSSL,
	})
	if err != nil {
		log.Fatal("初始化 MinIO 客户端失败", zap.Error(err))
	}

	publisher := event.NewRedisStreamPublisher(redisClient, log)

	return services{
		snippetSvc:     snippetsvc.NewSnippetService(snippetRepo, tagRepo, idgenClient, publisher, log),
		groupSvc:       groupsvc.NewGroupService(groupRepo, idgenClient, log),
		tagSvc:         tagsvc.NewTagService(tagRepo, idgenClient, log),
		templateSvc:    templatesvc.NewTemplateService(templateRepo, idgenClient, log),
		lineageSvc:     lineagesvc.NewService(lineageRepo, idgenClient, log),
		shareSvc:       sharesvc.NewService(shareRepo, snippetRepo, idgenClient, log),
		uploadSvc:      uploadsvc.NewUploadService(minioStorage, uploadsvc.Options{PresignExpiry: time.Duration(cfg.MinIO.PresignExpiry) * time.Second, MaxFileSize: cfg.MinIO.MaxUploadSize, AllowedMIMEs: cfg.MinIO.AllowedMIMEs}, log),
		aimetadataRepo: aimetadataRepo,
	}
}

func buildHandlers(svcs services, log *zap.Logger) handlers {
	return handlers{
		groupHandler:  handler.NewGroupHandler(svcs.groupSvc, log),
		shareHandler:  handler.NewShareHandler(svcs.shareSvc, log),
		uploadHandler: handler.NewUploadHandler(svcs.uploadSvc, log),
	}
}

func buildHealthChecker(entClient *ent.Client, redisClient *redis.Client) *health.Checker {
	checker := health.NewChecker()
	checker.AddCheck("postgres", func(ctx context.Context) error {
		_, err := entClient.Snippet.Query().Limit(1).Count(ctx)
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
