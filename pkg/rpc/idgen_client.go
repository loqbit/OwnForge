package rpc

import (
	"context"
	"fmt"
	"log"

	pb "github.com/ownforge/ownforge/pkg/proto/idgen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var idgenClient pb.IDGeneratorClient

// InitIDGenClient 初始化全局的 ID Generator gRPC 客户端
func InitIDGenClient(targetAddr string) error {
	conn, err := grpc.NewClient(
		targetAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin": {}}]}`),
	)
	if err != nil {
		return fmt.Errorf("无法连接到发号器服务 %s: %w", targetAddr, err)
	}

	idgenClient = pb.NewIDGeneratorClient(conn)
	log.Printf("ID Generator gRPC Client successfully connected to: %s", targetAddr)
	return nil
}

// GenerateID 向远端服务请求获取下一个雪花算法 ID
func GenerateID(ctx context.Context) (int64, error) {
	if idgenClient == nil {
		return 0, fmt.Errorf("ID Generator Client 未初始化")
	}

	resp, err := idgenClient.NextID(ctx, &pb.NextIDRequest{})
	if err != nil {
		return 0, fmt.Errorf("RPC 调用发号器失败: %w", err)
	}

	return resp.Id, nil
}
