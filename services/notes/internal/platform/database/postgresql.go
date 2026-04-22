package database

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonPG "github.com/ownforge/ownforge/pkg/postgres"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/migrate"

	"go.uber.org/zap"
)

// InitEntClient 初始化 Ent 客户端。
//
// 底层 Postgres 连接（含 OTel 追踪和连接池）由 common/postgres 统一管理，
// 本函数只负责 Ent 包装和 Schema Migration。
func InitEntClient(driver, source string, autoMigrate bool, log *zap.Logger) *ent.Client {
	db, err := commonPG.Init(commonPG.Config{
		Driver: driver,
		Source: source,
	}, commonPG.DefaultPoolConfig(), log)
	if err != nil {
		log.Fatal("初始化数据库失败", zap.Error(err))
		return nil
	}

	drv := sql.OpenDB(driver, db)
	client := ent.NewClient(ent.Driver(drv))

	if autoMigrate {
		if err := client.Schema.Create(
			context.Background(),
			migrate.WithDropIndex(true),
			migrate.WithDropColumn(true),
		); err != nil {
			log.Fatal("自动执行 Ent schema migration 失败", zap.Error(err))
			return nil
		}
		log.Info("已执行 Ent schema migration", zap.Bool("auto_migrate", true))
	} else {
		log.Info("跳过 Ent schema migration", zap.Bool("auto_migrate", false))
	}

	log.Info("成功初始化 Ent 客户端")
	return client
}
