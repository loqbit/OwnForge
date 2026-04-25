package database

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonPG "github.com/loqbit/ownforge/pkg/postgres"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	"github.com/loqbit/ownforge/services/user-platform/internal/ent/migrate"

	"go.uber.org/zap"
)

// InitEntClient initializes the Ent client.
//
// The underlying Postgres connection, including OTel tracing and pooling, is managed centrally by common/postgres,
// while this function only handles the Ent wrapper and schema migration, which cannot be shared because each service has its own schema.
func InitEntClient(driver, source string, autoMigrate bool, log *zap.Logger) *ent.Client {
	db, err := commonPG.Init(commonPG.Config{
		Driver: driver,
		Source: source,
	}, commonPG.DefaultPoolConfig(), log)
	if err != nil {
		log.Fatal("failed to initialize database", zap.Error(err))
		return nil
	}

	drv := sql.OpenDB(driver, db)
	client := ent.NewClient(ent.Driver(drv))

	if autoMigrate {
		if err := migrateLegacyOutboxJSONColumns(context.Background(), drv); err != nil {
			log.Fatal("failed to migrate event_outboxes JSON columns", zap.Error(err))
			return nil
		}
		if err := client.Schema.Create(
			context.Background(),
			migrate.WithDropIndex(true),
			migrate.WithDropColumn(true),
		); err != nil {
			log.Fatal("failed to run Ent schema migration automatically", zap.Error(err))
			return nil
		}
		log.Info("Ent schema migration completed", zap.Bool("auto_migrate", true))
	} else {
		log.Info("skipped Ent schema migration", zap.Bool("auto_migrate", false))
	}

	log.Info("Ent client initialized successfully")
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
