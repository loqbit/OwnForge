package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCLogFieldExtractor func(context.Context) []zap.Field

// GRPCUnaryServerInterceptor 统一记录 gRPC 一元请求的访问日志。
// 会自动从 ctx 中提取 trace_id / span_id，并允许服务方补充自定义字段。
func GRPCUnaryServerInterceptor(log *zap.Logger, extraFields GRPCLogFieldExtractor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)

		reqLog := Ctx(ctx, log)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
		}
		if extraFields != nil {
			fields = append(fields, extraFields(ctx)...)
		}

		if err == nil {
			fields = append(fields, zap.String("code", codes.OK.String()))
			reqLog.Info("gRPC 请求成功", fields...)
			return resp, nil
		}

		st, _ := status.FromError(err)
		fields = append(fields,
			zap.String("code", st.Code().String()),
			zap.Error(err),
		)

		switch st.Code() {
		case codes.Canceled, codes.DeadlineExceeded, codes.InvalidArgument, codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
			reqLog.Warn("gRPC 请求失败", fields...)
		default:
			reqLog.Error("gRPC 请求失败", fields...)
		}

		return resp, err
	}
}
