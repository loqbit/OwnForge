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

// userIDKey is the context key used to store the authenticated user ID.
const userIDKey contextKey = "user_id"

// authWhiteList contains public methods that do not require authentication.
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

// GatewayAuthInterceptor uses a gateway-trust model.
// It does not validate JWTs locally and instead trusts x-user-id injected by the gateway into gRPC metadata.
// This mirrors go-note's GatewayAuth middleware for the HTTP X-User-Id header.
func GatewayAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 1. Allowlisted methods do not require identity.
		if _, ok := authWhiteList[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		// 2. Read x-user-id passed by the gateway through gRPC metadata.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata not found")
		}

		values := md.Get("x-user-id")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "x-user-id not provided")
		}

		userID, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid x-user-id format")
		}

		// 3. Inject userID into the context for downstream handlers.
		newCtx := context.WithValue(ctx, userIDKey, userID)

		return handler(newCtx, req)
	}
}

// UserIDFromContext extracts userID from context for gRPC handlers.
func UserIDFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return 0, status.Error(codes.Unauthenticated, "UserID not found in context")
	}
	userID, ok := val.(int64)
	if !ok {
		return 0, status.Error(codes.Internal, "UserID in context has invalid type")
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
