package database

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonPG "github.com/luckysxx/common/postgres"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/migrate"

	"go.uber.org/zap"
)

// InitEntClient 初始化 Ent 客户端。
//
// 底层 Postgres 连接（含 OTel 追踪和连接池）由 common/postgres 统一管理，
// 本函数只负责 Ent 包装和 Schema Migration —— 这两部分因各服务 Schema 不同而无法通用化。
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
		if err := migrateLegacyOutboxJSONColumns(context.Background(), drv); err != nil {
			log.Fatal("迁移 event_outboxes JSON 列失败", zap.Error(err))
			return nil
		}
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

func migrateLegacyOutboxJSONColumns(ctx context.Context, drv *sql.Driver) error {
	query := `
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'event_outboxes'
      AND column_name = 'payload'
      AND udt_name = 'bytea'
  ) THEN
    ALTER TABLE public.event_outboxes
      ALTER COLUMN payload TYPE jsonb
      USING convert_from(payload, 'UTF8')::jsonb;
  END IF;

  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'event_outboxes'
      AND column_name = 'headers'
      AND udt_name = 'bytea'
  ) THEN
    ALTER TABLE public.event_outboxes
      ALTER COLUMN headers TYPE jsonb
      USING CASE
        WHEN headers IS NULL THEN NULL
        ELSE convert_from(headers, 'UTF8')::jsonb
      END;
  END IF;
END $$;
`
	_, err := drv.ExecContext(ctx, query)
	return err
}
