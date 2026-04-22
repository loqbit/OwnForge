package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/zap"
)

// Config 通用 Postgres 连接配置
type Config struct {
	Driver      string `mapstructure:"driver"`
	Source      string `mapstructure:"source"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
}

// PoolConfig 连接池参数（所有字段均提供合理默认值，各服务可按需覆盖）
type PoolConfig struct {
	MaxOpenConns    int           // 最大打开连接数（PG 默认 max_connections=100，单服务建议 ≤25）
	MaxIdleConns    int           // 最大空闲连接数（避免频繁建连的开销）
	ConnMaxLifetime time.Duration // 连接最大存活时间（防止被 PG 踢掉的僵尸连接）
	ConnMaxIdleTime time.Duration // 空闲连接回收时间（释放长时间不用的连接）
}

// DefaultPoolConfig 返回生产级默认连接池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 3 * time.Minute,
	}
}

// Init 初始化带 OTel 追踪和连接池的 *sql.DB
//
// 设计决策：返回 error 而非 log.Fatal，将「连接失败时是降级还是退出」的决策权交给调用方。
// 例如 go-chat 在 DB 不可用时会降级为内存仓储，而 user-platform 则需要直接退出。
func Init(cfg Config, pool PoolConfig, log *zap.Logger) (*sql.DB, error) {
	db, err := otelsql.Open(cfg.Driver, cfg.Source,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)

	// 连接验证
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}

	log.Info("成功连接到 PostgreSQL",
		zap.Int("max_open_conns", pool.MaxOpenConns),
		zap.Int("max_idle_conns", pool.MaxIdleConns),
		zap.Duration("conn_max_lifetime", pool.ConnMaxLifetime),
		zap.Duration("conn_max_idle_time", pool.ConnMaxIdleTime),
	)
	return db, nil
}
