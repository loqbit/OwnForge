package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config 定义了通用的 Redis 配置
type Config struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Init 根据配置初始化一个标准的 Redis 客户端
func Init(cfg Config, log *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,

		// 连接池配置
		PoolSize:        20,                // 最大连接数（默认 10*CPU，显式设置避免依赖运行环境）
		MinIdleConns:    5,                 // 最小空闲连接（预热，减少冷启动延迟）
		MaxIdleConns:    10,                // 最大空闲连接
		ConnMaxIdleTime: 5 * time.Minute,   // 空闲超时回收
		ConnMaxLifetime: 30 * time.Minute,  // 连接最大存活时间
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal("无法连接到 Redis", zap.Error(err))
		return nil
	}

	log.Info("成功连接到 Redis")
	return client
}
