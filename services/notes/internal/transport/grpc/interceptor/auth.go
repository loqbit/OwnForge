package interceptor

import (
	"context"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

// GatewayAuthInterceptor 信任网关在 gRPC metadata 中注入的 x-user-id。
func GatewayAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "未找到 metadata")
		}

		values := md.Get("x-user-id")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "未提供 x-user-id")
		}

		userID, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "x-user-id 格式无效")
		}

		return handler(context.WithValue(ctx, userIDKey, userID), req)
	}
}

func isPublicMethod(fullMethod string) bool {
	switch fullMethod {
	case "/note.NoteService/GetPublicSnippet",
		"/note.NoteService/GetPublicShareByToken":
		return true
	}

	return strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/")
}

// UserIDFromContext 从 context 中提取 userID，供 gRPC handler 使用。
func UserIDFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return 0, status.Error(codes.Unauthenticated, "上下文中不存在 UserID")
	}

	userID, ok := val.(int64)
	if !ok {
		return 0, status.Error(codes.Internal, "上下文中的 UserID 类型错误")
	}

	return userID, nil
}

func LogFieldsFromContext(ctx context.Context) []zap.Field {
	val := ctx.Value(userIDKey)
	if val == nil {
		return nil
	}

	userID, ok := val.(int64)
	if !ok {
		return nil
	}

	return []zap.Field{zap.Int64("user_id", userID)}
}
