package gwproxy

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	notepb "github.com/loqbit/ownforge/pkg/proto/note"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NewNoteMux creates and configures the note service gRPC-Gateway reverse-proxy mux.
//
// Key design:
//   - WithMetadata: extract userID from the X-User-Id header already injected by the JWT middleware,
//     and inject it into outgoing gRPC metadata to align with go-note's GatewayAuthInterceptor.
//   - The returned mux should be wrapped with WrapHandler before mounting on Gin routes so the envelope format stays compatible.
func NewNoteMux(ctx context.Context, noteGRPCAddr string) (*runtime.ServeMux, error) {
	mux := runtime.NewServeMux(
		// Extract the X-User-Id header from the HTTP request and convert it into outgoing gRPC metadata.
		// The JWT middleware already writes userID into the X-User-Id header in auth.go:56,
		// so it can be reused here without extra auth logic.
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			md := metadata.MD{}
			if uid := r.Header.Get("X-User-Id"); uid != "" {
				md.Set("x-user-id", uid)
			}
			return md
		}),
	)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	err := notepb.RegisterNoteServiceHandlerFromEndpoint(ctx, mux, noteGRPCAddr, opts)
	if err != nil {
		return nil, err
	}

	return mux, nil
}
