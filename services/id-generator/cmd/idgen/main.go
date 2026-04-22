package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/luckysxx/common/logger"
	"github.com/luckysxx/common/probe"
	pb "github.com/luckysxx/common/proto/idgen"
	"github.com/ownforge/ownforge/services/id-generator/internal/idgen"
	"github.com/ownforge/ownforge/services/id-generator/internal/platform/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedIDGeneratorServer
	log *zap.Logger
}

func (s *server) NextID(ctx context.Context, in *pb.NextIDRequest) (*pb.NextIDResponse, error) {
	id := idgen.NextID()
	// 发号器为极度高频接口，这里为保证性能不打印每条请求的日志
	return &pb.NextIDResponse{Id: id}, nil
}

func main() {
	// 1. 初始化结构化日志
	logg := logger.NewLogger("id-generator")
	defer logg.Sync()

	// 2. 加载 Viper 设置
	cfg := config.LoadConfig()

	// 3. 初始化雪花算法节点
	if err := idgen.Init(cfg.Snowflake.NodeID); err != nil {
		logg.Fatal("初始化雪花算法失败", zap.Error(err), zap.Int64("node_id", cfg.Snowflake.NodeID))
	}

	// 4. 监听端口
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logg.Fatal("无法建立监听端口", zap.Error(err), zap.String("addr", addr))
	}

	// 5. 组装并注册 gRPC
	s := grpc.NewServer()
	pb.RegisterIDGeneratorServer(s, &server{log: logg})
	reflection.Register(s)

	// 6. 异步启动与优雅停机
	go func() {
		logg.Info("ID Generator 服务已启动",
			zap.Int("port", cfg.Server.Port),
			zap.Int64("node_id", cfg.Snowflake.NodeID),
		)
		if err := s.Serve(lis); err != nil {
			logg.Fatal("发号器服务异常终止", zap.Error(err))
		}
	}()

	// 7. 探针管理端口：/healthz, /readyz, /metrics
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probeShutdown := probe.Serve(ctx, ":9096", logg)
	defer probeShutdown()

	// 拦截终止信号实现优雅停机
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logg.Info("收到进程终止信号，开始优雅停机...")
	s.GracefulStop()
	logg.Info("ID Generator 服务已安全退出")
}
