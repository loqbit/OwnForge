package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	grpchealth "google.golang.org/grpc/health"

	"github.com/luckysxx/common/logger"
	commonOtel "github.com/luckysxx/common/otel"
	"github.com/luckysxx/common/probe"
	commonRedis "github.com/luckysxx/common/redis"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
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

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	_ = godotenv.Load()

	log := logger.NewLogger("go-note-grpc")
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

	// 探针：独立管理端口 + gRPC Health 同步
	grpcHealthServer := grpchealth.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	probeShutdown := probe.Serve(ctx, ":"+cfg.Metrics.Port, log,
		probe.WithCheck("postgres", func(ctx context.Context) error {
			_, err := entClient.Snippet.Query().Limit(1).Count(ctx)
			return err
		}),
		probe.WithRedis(redisClient),
		probe.WithGRPCHealth(grpcHealthServer, "note.NoteService"),
	)
	defer probeShutdown()

	snippetSvc, groupSvc, tagSvc, templateSvc, lineageSvc, shareSvc, uploadSvc, aimetadataRepo := buildServices(cfg, entClient, idgenClient, log)

	// Seed 系统预置模板（幂等）
	if err := templateSvc.SeedSystemTemplates(context.Background()); err != nil {
		log.Error("seed 系统模板失败", zap.Error(err))
	}

	grpcServer := transportgrpc.SetupServer(snippetSvc, groupSvc, tagSvc, templateSvc, lineageSvc, shareSvc, uploadSvc, aimetadataRepo, grpcHealthServer, log)

	runServer(grpcServer, grpcHealthServer, cfg.GRPCServer.Port, log)
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

func buildServices(
	cfg *config.Config,
	entClient *ent.Client,
	idgenClient platformidgen.Client,
	log *zap.Logger,
) (snippetsvc.SnippetService, groupsvc.GroupService, tagsvc.TagService, templatesvc.TemplateService, lineagesvc.Service, sharesvc.Service, uploadsvc.UploadService, aimetadatarepo.Repository) {
	snippetRepo := snippetstore.New(entClient)
	aimetadataRepo := aimetadatastore.New(entClient)
	groupRepo := groupstore.New(entClient)
	tagRepo := tagstore.New(entClient)
	templateRepo := templatestore.New(entClient)
	lineageRepo := lineagestore.New(entClient)
	shareRepo := sharestore.New(entClient)

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

	return snippetsvc.NewSnippetService(snippetRepo, tagRepo, idgenClient, nil, log),
		groupsvc.NewGroupService(groupRepo, idgenClient, log),
		tagsvc.NewTagService(tagRepo, idgenClient, log),
		templatesvc.NewTemplateService(templateRepo, idgenClient, log),
		lineagesvc.NewService(lineageRepo, idgenClient, log),
		sharesvc.NewService(shareRepo, snippetRepo, idgenClient, log),
		uploadsvc.NewUploadService(minioStorage, uploadsvc.Options{
			PresignExpiry: time.Duration(cfg.MinIO.PresignExpiry) * time.Second,
			MaxFileSize:   cfg.MinIO.MaxUploadSize,
			AllowedMIMEs:  cfg.MinIO.AllowedMIMEs,
		}, log),
		aimetadataRepo
}

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
	probe.GRPCShutdown(healthServer, "note.NoteService")
	s.GracefulStop()

	log.Info("gRPC 服务已安全退出")
}
