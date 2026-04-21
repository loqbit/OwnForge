// Package lock 提供轻量的分布式互斥锁抽象。
//
// 当前唯一使用场景：防止多个 AI Worker 并发处理同一个 snippet 时重复调用 LLM。
// 不追求严格互斥（网络分区下可能出现双持），只追求"99% 情况下节省 LLM 成本"。
// 任何需要强一致的场景请使用业务侧幂等 hash 校验（content_hash）。
package lock

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotAcquired 表示锁已被他人持有。
var ErrNotAcquired = errors.New("lock: not acquired")

// Locker 分布式互斥锁接口。
//
// Phase 6 单机版可以实现一个基于 sync.Map + Mutex 的进程内版本，无需 Redis。
type Locker interface {
	// Acquire 尝试获取 key 对应的锁，ttl 是锁的最长持有时间（防死锁）。
	// 成功返回 release 函数；未获取到返回 ErrNotAcquired。
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func(), err error)
}

// RedisLocker 基于 Redis SET NX PX 实现。
type RedisLocker struct {
	client *redis.Client
	// token 用于确保 release 时只删除自己持有的锁（防止过期后误删他人锁）。
	// 实际 release 时会通过 Lua 脚本做 CAS。
}

// NewRedisLocker 创建一个 Redis 实现的 Locker。
func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{client: client}
}

// releaseScript: 仅当 value 与传入 token 相等时才 DEL，避免锁过期后误删他人的锁。
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

// Acquire 使用 SET NX PX 原子获取锁。
func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (func(), error) {
	token := randomToken()
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotAcquired
	}

	release := func() {
		// 用独立 ctx：即使原 ctx 已 cancel，也要尝试释放
		relCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = releaseScript.Run(relCtx, l.client, []string{key}, token).Result()
	}
	return release, nil
}
