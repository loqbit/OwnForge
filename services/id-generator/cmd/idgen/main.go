package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/loqbit/ownforge/pkg/logger"
	"github.com/loqbit/ownforge/pkg/probe"
	pb "github.com/loqbit/ownforge/pkg/proto/idgen"
	"github.com/loqbit/ownforge/services/id-generator/internal/idgen"
	"github.com/loqbit/ownforge/services/id-generator/internal/platform/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedIDGeneratorServer
	log *zap.Logger
}

func (s *server) NextID(ctx context.Context, in *pb.NextIDRequest) (*pb.NextIDResponse, error) {
	id, err := idgen.NextID(ctx)
	if err != nil {
		return nil, err
	}
	// The ID generator is an extremely high-frequency endpoint, so it does not log every request for performance reasons.
	return &pb.NextIDResponse{Id: id}, nil
}

func main() {
	// 1. initialize structured logging
	logg := logger.NewLogger("id-generator")
	defer logg.Sync()

	// 2. load Viper settings
	cfg := config.LoadConfig()

	// 3. initialize the Snowflake node
	if err := idgen.Init(cfg.Snowflake.NodeID); err != nil {
		logg.Fatal("failed to initialize Snowflake", zap.Error(err), zap.Int64("node_id", cfg.Snowflake.NodeID))
	}

	// 4. listen on the port
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logg.Fatal("failed to bind listening port", zap.Error(err), zap.String("addr", addr))
	}

	// 5. assemble and register gRPC
	s := grpc.NewServer()
	pb.RegisterIDGeneratorServer(s, &server{log: logg})
	reflection.Register(s)

	// 6. start asynchronously and shut down gracefully
	go func() {
		logg.Info("ID Generator service started",
			zap.Int("port", cfg.Server.Port),
			zap.Int64("node_id", cfg.Snowflake.NodeID),
		)
		if err := s.Serve(lis); err != nil {
			logg.Fatal("ID generator service terminated unexpectedly", zap.Error(err))
		}
	}()

	// 7. probe admin port: /healthz, /readyz, /metrics
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probeShutdown := probe.Serve(ctx, ":9096", logg)
	defer probeShutdown()

	// intercept termination signals to implement graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logg.Info("received process termination signal, starting graceful shutdown...")
	s.GracefulStop()
	logg.Info("ID Generator service exited safely")
}
