package database

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonPG "github.com/ownforge/ownforge/pkg/postgres"
	"github.com/ownforge/ownforge/services/notes/internal/ent"
	"github.com/ownforge/ownforge/services/notes/internal/ent/migrate"

	"go.uber.org/zap"
)

// InitEntClient initializes the Ent client.
//
// The underlying Postgres connection, including OTel tracing and pooling, is managed centrally by common/postgres.
// This function only handles the Ent wrapper and schema migration.
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
