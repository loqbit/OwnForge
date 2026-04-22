package idgen

import (
	"context"

	idgenpb "github.com/ownforge/ownforge/pkg/proto/idgen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client 定义 ID 生成器客户端能力。
type Client interface {
	NextID(ctx context.Context) (int64, error)
	Close() error
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client idgenpb.IDGeneratorClient
}

// New 创建一个基于 gRPC 的 ID 生成器客户端。
func New(addr string) (Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin": {}}]}`),
	)
	if err != nil {
		return nil, err
	}

	return &grpcClient{
		conn:   conn,
		client: idgenpb.NewIDGeneratorClient(conn),
	}, nil
}

func (c *grpcClient) NextID(ctx context.Context) (int64, error) {
	resp, err := c.client.NextID(ctx, &idgenpb.NextIDRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}
