// Package transportgrpc wires up the gRPC server, mirroring the HTTP router layer.
// It registers the interceptor chain and protobuf services.
package transportgrpc

import (
	"time"

	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	"github.com/loqbit/ownforge/pkg/metrics"
	auth_pb "github.com/loqbit/ownforge/pkg/proto/auth"
	user_pb "github.com/loqbit/ownforge/pkg/proto/user"
	accountservice "github.com/loqbit/ownforge/services/identity/internal/service/account"
	authservice "github.com/loqbit/ownforge/services/identity/internal/service/auth"
	"github.com/loqbit/ownforge/services/identity/internal/transport/grpc/interceptor"
	grpcserver "github.com/loqbit/ownforge/services/identity/internal/transport/grpc/server"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServerDependencies groups the dependencies required to build the gRPC server.
type ServerDependencies struct {
	UserService    accountservice.UserService
	ProfileService accountservice.ProfileService
	AuthService    authservice.AuthService
	HealthServer   healthgrpc.HealthServer
	Logger         *zap.Logger
}

// SetupServer builds the gRPC server.
func SetupServer(deps ServerDependencies) *grpc.Server {
	s := grpc.NewServer(
		// ── Keepalive ────────────────────────────────────────
		// Relax keepalive limits so the API Gateway can send pings every 10 seconds.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // Allow client pings as often as every 5 seconds.
			PermitWithoutStream: true,            // Allow pings without active streams.
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,  // Close idle connections after 5 minutes.
			MaxConnectionAge:      30 * time.Minute, // Cap connection lifetime at 30 minutes to help load balancing.
			MaxConnectionAgeGrace: 10 * time.Second, // Give in-flight RPCs 10 seconds to finish before closing.
			Time:                  15 * time.Second, // Send a server ping every 15 seconds.
			Timeout:               5 * time.Second,  // Treat the connection as dead after 5 seconds without a response.
		}),
		// ── Observability ─────────────────────────────────────
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			metrics.GRPCMetricsInterceptor(),
			interceptor.RecoveryInterceptor(deps.Logger),
			interceptor.GatewayAuthInterceptor(),
			commonlogger.GRPCUnaryServerInterceptor(deps.Logger, interceptor.LogFieldsFromContext),
		),
	)

	// Register protobuf services.
	user_pb.RegisterUserServiceServer(s, grpcserver.NewUserServer(grpcserver.UserServerDependencies{
		UserService:    deps.UserService,
		ProfileService: deps.ProfileService,
		Logger:         deps.Logger,
	}))
	auth_pb.RegisterAuthServiceServer(s, grpcserver.NewAuthServer(grpcserver.AuthServerDependencies{
		AuthService: deps.AuthService,
		Logger:      deps.Logger,
	}))

	// Register the native gRPC health service for standard Health/Check RPCs.
	healthgrpc.RegisterHealthServer(s, deps.HealthServer)

	return s
}
