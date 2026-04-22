// Package transportgrpc 组装 gRPC 服务端，对标 HTTP 层的 router.go。
// 职责：注册拦截器链 + 注册 Protobuf 服务。
package transportgrpc

import (
	"time"

	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/pkg/metrics"
	auth_pb "github.com/ownforge/ownforge/pkg/proto/auth"
	user_pb "github.com/ownforge/ownforge/pkg/proto/user"
	accountservice "github.com/ownforge/ownforge/services/user-platform/internal/service/account"
	authservice "github.com/ownforge/ownforge/services/user-platform/internal/service/auth"
	"github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/interceptor"
	grpcserver "github.com/ownforge/ownforge/services/user-platform/internal/transport/grpc/server"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServerDependencies 描述 gRPC 服务端组装所需的依赖集合。
type ServerDependencies struct {
	UserService    accountservice.UserService
	ProfileService accountservice.ProfileService
	AuthService    authservice.AuthService
	HealthServer   healthgrpc.HealthServer
	Logger         *zap.Logger
}

// SetupServer 组装 gRPC 服务端。
func SetupServer(deps ServerDependencies) *grpc.Server {
	s := grpc.NewServer(
		// ── Keepalive ────────────────────────────────────────
		// 放宽心跳限制，允许 API Gateway 以 10 秒间隔发送 ping。
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // 允许客户端最小 5 秒 ping 一次
			PermitWithoutStream: true,            // 允许无活跃流时的 ping
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,  // 空闲连接 5 分钟后关闭
			MaxConnectionAge:      30 * time.Minute, // 连接最长存活 30 分钟，促进负载均衡
			MaxConnectionAgeGrace: 10 * time.Second, // 关闭前给 10 秒完成正在执行的 RPC
			Time:                  15 * time.Second, // 服务端每 15 秒向客户端发 ping
			Timeout:               5 * time.Second,  // 5 秒没响应视为断连
		}),
		// ── 可观测性 ───────────────────────────────────────────
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			metrics.GRPCMetricsInterceptor(),
			interceptor.RecoveryInterceptor(deps.Logger),
			interceptor.GatewayAuthInterceptor(),
			commonlogger.GRPCUnaryServerInterceptor(deps.Logger, interceptor.LogFieldsFromContext),
		),
	)

	// 注册 Protobuf 服务
	user_pb.RegisterUserServiceServer(s, grpcserver.NewUserServer(grpcserver.UserServerDependencies{
		UserService:    deps.UserService,
		ProfileService: deps.ProfileService,
		Logger:         deps.Logger,
	}))
	auth_pb.RegisterAuthServiceServer(s, grpcserver.NewAuthServer(grpcserver.AuthServerDependencies{
		AuthService: deps.AuthService,
		Logger:      deps.Logger,
	}))

	// 注册 gRPC 原生健康检查服务，供标准 Health/Check RPC 调用。
	healthgrpc.RegisterHealthServer(s, deps.HealthServer)

	return s
}
