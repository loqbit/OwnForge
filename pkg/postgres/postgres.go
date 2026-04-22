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

// Config is the common Postgres connection configuration.
type Config struct {
	Driver      string `mapstructure:"driver"`
	Source      string `mapstructure:"source"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
}

// PoolConfig contains pool parameters with sensible defaults that services can override as needed.
type PoolConfig struct {
	MaxOpenConns    int           // maximum open connections (Postgres defaults max_connections to 100; a single service should usually stay <=25)
	MaxIdleConns    int           // maximum idle connections (to avoid frequent reconnect overhead)
	ConnMaxLifetime time.Duration // maximum connection lifetime (to avoid zombie connections dropped by Postgres)
	ConnMaxIdleTime time.Duration // idle connection cleanup time (to release long-unused connections)
}

// DefaultPoolConfig returns production-grade default connection-pool settings
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 3 * time.Minute,
	}
}

// Init initializes a *sql.DB with OTel tracing and pool settings.
//
// Design decision: return an error instead of calling log.Fatal so the caller can decide whether to degrade or exit on connection failure.
// For example, go-chat may fall back to in-memory storage when the DB is unavailable, while user-platform should exit directly.
func Init(cfg Config, pool PoolConfig, log *zap.Logger) (*sql.DB, error) {
	db, err := otelsql.Open(cfg.Driver, cfg.Source,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)

	// connection verification
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database connection verification failed: %w", err)
	}

	log.Info("connected to PostgreSQL successfully",
		zap.Int("max_open_conns", pool.MaxOpenConns),
		zap.Int("max_idle_conns", pool.MaxIdleConns),
		zap.Duration("conn_max_lifetime", pool.ConnMaxLifetime),
		zap.Duration("conn_max_idle_time", pool.ConnMaxIdleTime),
	)
	return db, nil
}
