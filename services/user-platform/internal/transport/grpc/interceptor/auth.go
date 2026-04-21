package interceptor

import (
	"context"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// 定义存储在 Context 中的 Key
const userIDKey contextKey = "user_id"

// authWhiteList 定义无需鉴权的公共接口白名单
var authWhiteList = map[string]bool{
	"/user.AuthService/Login":              true,
	"/user.AuthService/RefreshToken":       true,
	"/user.AuthService/VerifyToken":        true,
	"/user.AuthService/ExchangeSSO":        true,
	"/user.AuthService/SendPhoneCode":      true,
	"/user.AuthService/PhoneAuthEntry":     true,
	"/user.AuthService/PhonePasswordLogin": true,
	"/user.UserService/Register":           true,
}

// GatewayAuthInterceptor 网关信任模式拦截器。
// 不再自行验证 JWT，而是信任网关在 gRPC metadata 中注入的 x-user-id。
// 这与 go-note 的 GatewayAuth（读 HTTP X-User-Id header）保持对称。
func GatewayAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 1. 白名单放行：公共接口无需身份信息
		if _, ok := authWhiteList[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		// 2. 从 gRPC metadata 读取网关传递的 x-user-id
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

		// 3. 将 userID 注入 context，下游 handler 通过 UserIDFromContext 取用
		newCtx := context.WithValue(ctx, userIDKey, userID)

		return handler(newCtx, req)
	}
}

// UserIDFromContext 从 context 中提取 userID，供 gRPC handler 调用。
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
