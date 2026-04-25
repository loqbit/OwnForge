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
	"github.com/loqbit/ownforge/pkg/health"
	"github.com/loqbit/ownforge/pkg/logger"
	commonOtel "github.com/loqbit/ownforge/pkg/otel"
	"github.com/loqbit/ownforge/pkg/probe"
	notepb "github.com/loqbit/ownforge/pkg/proto/note"
	commonRedis "github.com/loqbit/ownforge/pkg/redis"
	"github.com/loqbit/ownforge/services/notes/internal/ent"
	"github.com/loqbit/ownforge/services/notes/internal/event"
	"github.com/loqbit/ownforge/services/notes/internal/platform/config"
	"github.com/loqbit/ownforge/services/notes/internal/platform/database"
	platformidgen "github.com/loqbit/ownforge/services/notes/internal/platform/idgen"
	"github.com/loqbit/ownforge/services/notes/internal/platform/storage"
	aimetadatarepo "github.com/loqbit/ownforge/services/notes/internal/repository/aimetadata"
	groupsvc "github.com/loqbit/ownforge/services/notes/internal/service/group"
	lineagesvc "github.com/loqbit/ownforge/services/notes/internal/service/lineage"
	sharesvc "github.com/loqbit/ownforge/services/notes/internal/service/share"
	snippetsvc "github.com/loqbit/ownforge/services/notes/internal/service/snippet"
	tagsvc "github.com/loqbit/ownforge/services/notes/internal/service/tag"
	templatesvc "github.com/loqbit/ownforge/services/notes/internal/service/template"
	uploadsvc "github.com/loqbit/ownforge/services/notes/internal/service/upload"
	aimetadatastore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/aimetadata"
	groupstore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/group"
	lineagestore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/lineage"
	sharestore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/share"
	snippetstore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/snippet"
	tagstore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/tag"
	templatestore "github.com/loqbit/ownforge/services/notes/internal/store/entstore/template"
	transportgrpc "github.com/loqbit/ownforge/services/notes/internal/transport/grpc"
	httpgwproxy "github.com/loqbit/ownforge/services/notes/internal/transport/http/server/gwproxy"
	"github.com/loqbit/ownforge/services/notes/internal/transport/http/server/handler"
	httprouter "github.com/loqbit/ownforge/services/notes/internal/transport/http/server/router"
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
		log.Fatal("failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer otelShutdown(context.Background())

	svcs := buildServices(cfg, redisClient, entClient, idgenClient, log)
	hs := buildHandlers(svcs, log)

	if err := svcs.templateSvc.SeedSystemTemplates(context.Background()); err != nil {
		log.Error("failed to seed system templates", zap.Error(err))
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
		log.Fatal("failed to listen on gRPC port", zap.Error(err))
	}

	go func() {
		log.Info("gRPC server started", zap.String("port", cfg.GRPCServer.Port))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("gRPC server terminated unexpectedly", zap.Error(err))
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
		log.Fatal("failed to register grpc-gateway", zap.Error(err))
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
	probe.GRPCShutdown(grpcHealthServer, "note.NoteService")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shut down HTTP server", zap.Error(err))
	}

	grpcServer.GracefulStop()
	log.Info("go-note single-process server exited safely")
}

func initInfra(cfg *config.Config, log *zap.Logger) (*ent.Client, *redis.Client, platformidgen.Client) {
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	redisClient := commonRedis.Init(cfg.Redis, log)
	idgenClient, err := platformidgen.New(cfg.IDGenerator.Addr)
	if err != nil {
		log.Fatal("failed to initialize id-generator client", zap.Error(err))
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
		log.Fatal("failed to initialize MinIO client", zap.Error(err))
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
