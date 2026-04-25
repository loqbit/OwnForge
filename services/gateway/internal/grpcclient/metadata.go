package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
)

// WithUserID injects userID into outgoing gRPC metadata.
// The downstream identity GatewayAuthInterceptor reads identity information from "x-user-id".
// This ensures that internal microservices pass only identity markers instead of raw JWTs, reducing token exposure.
func WithUserID(ctx context.Context, userID int64) context.Context {
	md := metadata.Pairs("x-user-id", fmt.Sprintf("%d", userID))
	return metadata.NewOutgoingContext(ctx, md)
}
