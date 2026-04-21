package phonecodestore

import (
	"context"
	"fmt"
	"time"

	sessionrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/session"
	"github.com/redis/go-redis/v9"
)

// PhoneCodeStore 是 PhoneCodeRepository 的 Redis 实现。
type PhoneCodeStore struct {
	cli *redis.Client
}

// NewPhoneCodeStore 创建一个 PhoneCodeRepository 实例，直接持有 redis.Client。
func NewPhoneCodeStore(cli *redis.Client) sessionrepo.PhoneCodeRepository {
	return &PhoneCodeStore{cli: cli}
}

func (s *PhoneCodeStore) SaveCode(ctx context.Context, phone string, scene string, code string, ttl time.Duration, cooldown time.Duration) error {
	pipe := s.cli.TxPipeline()
	pipe.Set(ctx, phoneCodeKey(scene, phone), code, ttl)
	pipe.Set(ctx, phoneCodeCooldownKey(scene, phone), "1", cooldown)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *PhoneCodeStore) CooldownTTL(ctx context.Context, phone string, scene string) (time.Duration, bool, error) {
	ttl, err := s.cli.TTL(ctx, phoneCodeCooldownKey(scene, phone)).Result()
	if err != nil {
		return 0, false, err
	}
	if ttl <= 0 {
		return 0, false, nil
	}
	return ttl, true, nil
}

func (s *PhoneCodeStore) VerifyCode(ctx context.Context, phone string, scene string, code string) (bool, error) {
	saved, err := s.cli.Get(ctx, phoneCodeKey(scene, phone)).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return saved == code, nil
}

func (s *PhoneCodeStore) DeleteCode(ctx context.Context, phone string, scene string) error {
	return s.cli.Del(ctx, phoneCodeKey(scene, phone)).Err()
}

func (s *PhoneCodeStore) SaveCooldown(ctx context.Context, phone string, scene string, cooldown time.Duration) error {
	return s.cli.Set(ctx, phoneCodeCooldownKey(scene, phone), "1", cooldown).Err()
}

func (s *PhoneCodeStore) SaveBizID(ctx context.Context, phone string, scene string, bizID string, ttl time.Duration) error {
	return s.cli.Set(ctx, phoneCodeBizIDKey(scene, phone), bizID, ttl).Err()
}

func (s *PhoneCodeStore) GetBizID(ctx context.Context, phone string, scene string) (string, bool, error) {
	value, err := s.cli.Get(ctx, phoneCodeBizIDKey(scene, phone)).Result()
	if err != nil {
		if err == redis.Nil {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (s *PhoneCodeStore) DeleteBizID(ctx context.Context, phone string, scene string) error {
	return s.cli.Del(ctx, phoneCodeBizIDKey(scene, phone)).Err()
}

func phoneCodeKey(scene string, phone string) string {
	return fmt.Sprintf("phone_code:%s:%s", scene, phone)
}

func phoneCodeCooldownKey(scene string, phone string) string {
	return fmt.Sprintf("phone_code_cd:%s:%s", scene, phone)
}

func phoneCodeBizIDKey(scene string, phone string) string {
	return fmt.Sprintf("phone_code_bizid:%s:%s", scene, phone)
}
