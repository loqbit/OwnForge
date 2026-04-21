package interceptor

import (
	"context"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor 返回一个一元服务器拦截器，用于拦截 Handler 中发生的 Panic，
// 将其转换为标准的 gRPC Internal 错误返回给客户端，并记录堆栈日志，防止服务崩溃。
func RecoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("gRPC 拦截器捕获到 Panic",
					zap.String("method", info.FullMethod),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				// 转换成 gRPC 的 codes.Internal 错误
				err = status.Errorf(codes.Internal, "Internal server error")
			}
		}()

		return handler(ctx, req)
	}
}
