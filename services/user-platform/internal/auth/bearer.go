package auth

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMissingAuthHeader       = errors.New("missing authorization header")
	ErrInvalidAuthHeaderFormat = errors.New("invalid authorization header format")
	ErrInvalidOrExpiredToken   = errors.New("invalid or expired token")
)

// ExtractBearerToken 解析并校验 Bearer 格式的 Authorization 请求头。
func ExtractBearerToken(authHeader string) (string, error) {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthHeaderFormat
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrInvalidAuthHeaderFormat
	}

	return token, nil
}

// AuthenticateBearerToken 从 Authorization 请求头中提取令牌并返回用户 ID。
func AuthenticateBearerToken(jwtManager *JWTManager, authHeader string) (int64, error) {
	if jwtManager == nil {
		return 0, errors.New("jwt manager is nil")
	}

	token, err := ExtractBearerToken(authHeader)
	if err != nil {
		return 0, err
	}

	claims, err := jwtManager.VerifyToken(token)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidOrExpiredToken, err)
	}

	return claims.UserID, nil
}
