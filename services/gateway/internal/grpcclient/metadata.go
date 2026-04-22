package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
)

// WithUserID 将 userID 注入到 gRPC outgoing metadata 中。
// 下游 user-platform 的 GatewayAuthInterceptor 会从 "x-user-id" 读取身份信息。
// 这保证了内网微服务之间只传递身份标识，不传递原始 JWT Token，减少密钥泄露面。
func WithUserID(ctx context.Context, userID int64) context.Context {
	md := metadata.Pairs("x-user-id", fmt.Sprintf("%d", userID))
	return metadata.NewOutgoingContext(ctx, md)
}
