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

// GRPCUnaryServerInterceptor records access logs uniformly for gRPC unary requests.
// It automatically extracts trace_id / span_id from ctx and lets services add custom fields.
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
			reqLog.Info("gRPC requestsuccess", fields...)
			return resp, nil
		}

		st, _ := status.FromError(err)
		fields = append(fields,
			zap.String("code", st.Code().String()),
			zap.Error(err),
		)

		switch st.Code() {
		case codes.Canceled, codes.DeadlineExceeded, codes.InvalidArgument, codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
			reqLog.Warn("gRPC request failed", fields...)
		default:
			reqLog.Error("gRPC request failed", fields...)
		}

		return resp, err
	}
}
