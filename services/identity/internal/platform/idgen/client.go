package idgen

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	pkgid "github.com/loqbit/ownforge/pkg/id"
	idgenpb "github.com/loqbit/ownforge/pkg/proto/idgen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	envIDSource = "ID_SOURCE"
	envIDNodeID = "ID_NODE_ID"

	sourceLocal = "local"
	sourceGRPC  = "grpc"
)

// Client generates int64 IDs for identity.
type Client interface {
	NextID(ctx context.Context) (int64, error)
	Close() error
}

type localClient struct {
	gen pkgid.Generator
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client idgenpb.IDGeneratorClient
}

// New creates an ID generator client selected by ID_SOURCE.
func New(addr string) (Client, error) {
	switch sourceFromEnv() {
	case sourceGRPC:
		return newGRPCClient(addr)
	default:
		return newLocalClient()
	}
}

func newLocalClient() (Client, error) {
	nodeID, err := nodeIDFromEnv()
	if err != nil {
		return nil, err
	}

	gen, err := pkgid.NewLocalSnowflake(nodeID)
	if err != nil {
		return nil, err
	}
	return &localClient{gen: gen}, nil
}

func newGRPCClient(addr string) (Client, error) {
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

func (c *localClient) NextID(ctx context.Context) (int64, error) {
	_, _ = tenantIDFromContext(ctx)
	return c.gen.NextID(ctx)
}

func (c *localClient) Close() error {
	return c.gen.Close()
}

func (c *grpcClient) NextID(ctx context.Context) (int64, error) {
	if tenantID, ok := tenantIDFromContext(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, "tenant_id", tenantID)
	}

	resp, err := c.client.NextID(ctx, &idgenpb.NextIDRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

func sourceFromEnv() string {
	source := strings.ToLower(strings.TrimSpace(os.Getenv(envIDSource)))
	if source == "" {
		return sourceLocal
	}
	if source == sourceGRPC {
		return sourceGRPC
	}
	return sourceLocal
}

func nodeIDFromEnv() (int64, error) {
	raw := strings.TrimSpace(os.Getenv(envIDNodeID))
	if raw == "" {
		return 0, nil
	}

	nodeID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", envIDNodeID, err)
	}
	return nodeID, nil
}

func tenantIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}

	if tenantID, ok := ctx.Value("tenant_id").(string); ok && tenantID != "" {
		return tenantID, true
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("tenant_id"); len(values) > 0 && values[0] != "" {
			return values[0], true
		}
	}

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if values := md.Get("tenant_id"); len(values) > 0 && values[0] != "" {
			return values[0], true
		}
	}

	return "", false
}
