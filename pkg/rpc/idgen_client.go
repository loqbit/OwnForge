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

// InitIDGenClient initializes the global ID Generator gRPC client.
func InitIDGenClient(targetAddr string) error {
	conn, err := grpc.NewClient(
		targetAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin": {}}]}`),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to the ID generator service %s: %w", targetAddr, err)
	}

	idgenClient = pb.NewIDGeneratorClient(conn)
	log.Printf("ID Generator gRPC Client successfully connected to: %s", targetAddr)
	return nil
}

// GenerateID requests the next Snowflake ID from the remote service.
func GenerateID(ctx context.Context) (int64, error) {
	if idgenClient == nil {
		return 0, fmt.Errorf("ID Generator client is not initialized")
	}

	resp, err := idgenClient.NextID(ctx, &pb.NextIDRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to call the ID generator via RPC: %w", err)
	}

	return resp.Id, nil
}
