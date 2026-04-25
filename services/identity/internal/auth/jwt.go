package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token lifetime constants.
const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

// Claims is the JWT claims payload.
type Claims struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	UserVersion int64  `json:"user_version"`
	jwt.RegisteredClaims
}

// JWTManager manages JWT operations.
type JWTManager struct {
	secret []byte
}

// NewJWTManager creates a JWT manager.
func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
	}
}

// GenerateAccessToken issues an access JWT.
func (j *JWTManager) GenerateAccessToken(userID int64, username string, userVersion int64) (string, error) {
	claims := Claims{
		UserID:      userID,
		Username:    username,
		UserVersion: userVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenDuration)),
			Issuer:    "identity",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// VerifyToken parses and validates a JWT.
func (j *JWTManager) VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	err = errors.New("invalid token")
	return nil, err
}
