package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config defines common Redis configuration.
type Config struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Init initializes a standard Redis client from the configuration.
func Init(cfg Config, log *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,

		// Connection pool settings
		PoolSize:        20,               // maximum connections (default is 10*CPU; explicitly set to avoid depending on the runtime environment)
		MinIdleConns:    5,                // minimum idle connections (pre-warms the pool to reduce cold-start latency)
		MaxIdleConns:    10,               // maximum idle connections
		ConnMaxIdleTime: 5 * time.Minute,  // idle-time cleanup
		ConnMaxLifetime: 30 * time.Minute, // maximum connection lifetime
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
		return nil
	}

	log.Info("connected to Redis successfully")
	return client
}
