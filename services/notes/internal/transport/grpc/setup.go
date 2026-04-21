// Package transportgrpc 组装 gRPC Server，对标 HTTP 层的 router.go。
package transportgrpc

import (
	"time"

	commonlogger "github.com/luckysxx/common/logger"
	"github.com/luckysxx/common/metrics"
	notepb "github.com/luckysxx/common/proto/note"
	aimetadatarepo "github.com/ownforge/ownforge/services/notes/internal/repository/aimetadata"
	groupsvc "github.com/ownforge/ownforge/services/notes/internal/service/group"
	lineagesvc "github.com/ownforge/ownforge/services/notes/internal/service/lineage"
	sharesvc "github.com/ownforge/ownforge/services/notes/internal/service/share"
	snippetsvc "github.com/ownforge/ownforge/services/notes/internal/service/snippet"
	tagsvc "github.com/ownforge/ownforge/services/notes/internal/service/tag"
	templatesvc "github.com/ownforge/ownforge/services/notes/internal/service/template"
	uploadsvc "github.com/ownforge/ownforge/services/notes/internal/service/upload"
	"github.com/ownforge/ownforge/services/notes/internal/transport/grpc/interceptor"
	grpcserver "github.com/ownforge/ownforge/services/notes/internal/transport/grpc/server"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// SetupServer 组装 gRPC Server（对标 HTTP 的 SetupRouter）
func SetupServer(
	snippetSvc snippetsvc.SnippetService,
	groupSvc groupsvc.GroupService,
	tagSvc tagsvc.TagService,
	templateSvc templatesvc.TemplateService,
	lineageSvc lineagesvc.Service,
	shareSvc sharesvc.Service,
	uploadSvc uploadsvc.UploadService,
	aimetadataRepo aimetadatarepo.Repository,
	healthServer healthgrpc.HealthServer,
	log *zap.Logger,
) *grpc.Server {
	const maxMsgSize = 16 << 20 // 16 MB — 支持最大 10MB 文件上传 + protobuf 编码开销

	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		// ── Keepalive ────────────────────────────────────────
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 10 * time.Second,
			Time:                  15 * time.Second,
			Timeout:               5 * time.Second,
		}),
		// ── Observability ────────────────────────────────────
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			metrics.GRPCMetricsInterceptor(),
			interceptor.GatewayAuthInterceptor(),
			commonlogger.GRPCUnaryServerInterceptor(log, interceptor.LogFieldsFromContext),
		),
	)

	notepb.RegisterNoteServiceServer(s, grpcserver.NewNoteServer(snippetSvc, groupSvc, tagSvc, templateSvc, lineageSvc, shareSvc, uploadSvc, aimetadataRepo, log))
	healthgrpc.RegisterHealthServer(s, healthServer)

	return s
}
