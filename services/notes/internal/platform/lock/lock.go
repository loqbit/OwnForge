// Package lock provides a lightweight distributed mutex abstraction.
//
// The current use case is preventing duplicate LLM calls when multiple AI workers process the same snippet concurrently.
// It does not aim for strict mutual exclusion under network partitions; the goal is simply to save LLM cost in the vast majority of cases.
// Any path requiring strong consistency should rely on business-level idempotency checks such as content_hash.
package lock

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotAcquired means the lock is already held by someone else.
var ErrNotAcquired = errors.New("lock: not acquired")

// Locker is the distributed mutex interface.
//
// In a future single-node edition, this could be replaced with an in-process implementation based on sync.Map plus Mutex, without Redis.
type Locker interface {
	// Acquire tries to obtain the lock for the given key. ttl is the maximum lock hold time and prevents deadlocks.
	// On success it returns a release function; otherwise it returns ErrNotAcquired.
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func(), err error)
}

// RedisLocker is implemented with Redis SET NX PX.
type RedisLocker struct {
	client *redis.Client
	// token ensures that release only deletes the lock held by this caller, preventing accidental deletion after expiry and re-acquisition by someone else.
	// Actual release uses a Lua script for compare-and-swap semantics.
}

// NewRedisLocker creates a Redis-backed Locker.
func NewRedisLocker(client *redis.Client) *RedisLocker {
	return &RedisLocker{client: client}
}

// releaseScript deletes the key only when its value matches the supplied token, preventing accidental deletion of another holder's lock after expiry.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

// Acquire uses SET NX PX to obtain the lock atomically.
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
		// Use a separate context so release is still attempted even if the original context has been canceled.
		relCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = releaseScript.Run(relCtx, l.client, []string{key}, token).Result()
	}
	return release, nil
}
