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

// ExtractBearerToken parses and validates a Bearer Authorization request header.
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

// AuthenticateBearerToken extracts the token from the Authorization request header and returns the user ID.
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
