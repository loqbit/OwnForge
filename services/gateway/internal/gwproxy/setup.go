package gwproxy

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	notepb "github.com/ownforge/ownforge/pkg/proto/note"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NewNoteMux 创建并配置笔记服务的 gRPC-Gateway 反向代理 Mux。
//
// 关键设计:
//   - WithMetadata: 从 JWT 中间件已注入的 X-User-Id header 提取 userID，
//     注入 gRPC outgoing metadata，与 go-note 的 GatewayAuthInterceptor 对齐。
//   - 返回的 Mux 应通过 WrapHandler 包装后挂载到 Gin 路由，以保证信封格式兼容。
func NewNoteMux(ctx context.Context, noteGRPCAddr string) (*runtime.ServeMux, error) {
	mux := runtime.NewServeMux(
		// 从 HTTP 请求中提取 X-User-Id header，转换为 gRPC outgoing metadata。
		// JWT 中间件在 auth.go:56 已将 userID 写入 X-User-Id header，
		// 这里直接复用，无需额外鉴权逻辑。
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
