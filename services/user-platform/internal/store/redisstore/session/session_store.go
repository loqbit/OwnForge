package sessionstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	"github.com/redis/go-redis/v9"
)

// SessionStore is the Redis-backed implementation of SessionRepository.
type SessionStore struct {
	cli *redis.Client
}

const sessionFieldSeparator = "|"

// NewSessionStore creates a SessionRepository backed directly by redis.Client.
func NewSessionStore(cli *redis.Client) sessionrepo.SessionRepository {
	return &SessionStore{cli: cli}
}

func (s *SessionStore) SaveDeviceSession(ctx context.Context, userID int64, appCode string, deviceID string, refreshToken string, duration time.Duration) error {
	hashKey := fmt.Sprintf("user_sessions:%d", userID)
	fieldKey := sessionFieldKey(appCode, deviceID)

	// Clean up the old session reverse index to avoid orphaned keys after re-login.
	if oldToken, err := s.cli.HGet(ctx, hashKey, fieldKey).Result(); err == nil && oldToken != "" {
		s.cli.Del(ctx, fmt.Sprintf("refresh_token:%s", oldToken))
	}

	pipe := s.cli.TxPipeline()

	// Reverse index.
	redisKey := fmt.Sprintf("refresh_token:%s", refreshToken)
	val := sessionTokenValue(userID, appCode, deviceID)
	pipe.Set(ctx, redisKey, val, duration)

	// Forward hash index.
	pipe.HSet(ctx, hashKey, fieldKey, refreshToken)
	pipe.Expire(ctx, hashKey, duration)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis transaction failed: %w", err)
	}
	return nil
}

func (s *SessionStore) GetSessionByToken(ctx context.Context, refreshToken string) (int64, string, string, error) {
	redisKey := fmt.Sprintf("refresh_token:%s", refreshToken)
	val, err := s.cli.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, "", "", sharedrepo.ErrInvalidOrExpiredToken
		}
		return 0, "", "", fmt.Errorf("redis get failed: %w", err)
	}

	parts := strings.SplitN(val, sessionFieldSeparator, 3)
	if len(parts) != 3 {
		return 0, "", "", fmt.Errorf("invalid format in redis: %s", val)
	}

	userID, parseErr := strconv.ParseInt(parts[0], 10, 64)
	if parseErr != nil {
		return 0, "", "", fmt.Errorf("invalid user id in redis: %w", parseErr)
	}
	return userID, parts[1], parts[2], nil
}

func (s *SessionStore) ValidateDeviceToken(ctx context.Context, userID int64, appCode string, deviceID string, candidateToken string) error {
	hashKey := fmt.Sprintf("user_sessions:%d", userID)
	savedToken, err := s.cli.HGet(ctx, hashKey, sessionFieldKey(appCode, deviceID)).Result()
	if err != nil || savedToken != candidateToken {
		return sharedrepo.ErrInvalidOrExpiredToken
	}
	return nil
}

func (s *SessionStore) DeleteTokenIndex(ctx context.Context, refreshToken string) error {
	return s.cli.Del(ctx, fmt.Sprintf("refresh_token:%s", refreshToken)).Err()
}

func (s *SessionStore) DeleteAppSession(ctx context.Context, userID int64, appCode string, deviceID string) (string, error) {
	hashKey := fmt.Sprintf("user_sessions:%d", userID)
	fieldKey := sessionFieldKey(appCode, deviceID)
	oldToken, err := s.cli.HGet(ctx, hashKey, fieldKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("redis hget failed: %w", err)
	}

	pipe := s.cli.TxPipeline()
	pipe.HDel(ctx, hashKey, fieldKey)
	if oldToken != "" {
		pipe.Del(ctx, fmt.Sprintf("refresh_token:%s", oldToken))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("redis transaction failed: %w", err)
	}
	return oldToken, nil
}

func (s *SessionStore) TryLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return s.cli.SetNX(ctx, key, "locked", expiration).Result()
}

func (s *SessionStore) UnLock(ctx context.Context, key string) error {
	return s.cli.Del(ctx, key).Err()
}

func (s *SessionStore) SaveGracePeriod(ctx context.Context, oldToken string, newToken sessionrepo.TokenPair, duration time.Duration) error {
	val := fmt.Sprintf("%s|%s", newToken.AccessToken, newToken.RefreshToken)
	return s.cli.Set(ctx, fmt.Sprintf("grace_period:%s", oldToken), val, duration).Err()
}

func (s *SessionStore) CheckGracePeriod(ctx context.Context, oldToken string) (*sessionrepo.TokenPair, bool) {
	val, err := s.cli.Get(ctx, fmt.Sprintf("grace_period:%s", oldToken)).Result()
	if err != nil {
		return nil, false
	}

	parts := strings.Split(val, "|")
	if len(parts) == 2 {
		return &sessionrepo.TokenPair{
			AccessToken:  parts[0],
			RefreshToken: parts[1],
		}, true
	}
	return nil, false
}

func sessionFieldKey(appCode string, deviceID string) string {
	return appCode + sessionFieldSeparator + deviceID
}

func sessionTokenValue(userID int64, appCode string, deviceID string) string {
	return fmt.Sprintf("%d%s%s%s%s", userID, sessionFieldSeparator, appCode, sessionFieldSeparator, deviceID)
}
