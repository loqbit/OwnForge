package main

import (
	"context"

	"github.com/luckysxx/common/logger"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/bootstrap"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/database"
)

func main() {
	log := logger.NewLogger("user-seed-apps")
	defer log.Sync()

	cfg := config.LoadConfig()
	entClient := database.InitEntClient(cfg.Database.Driver, cfg.Database.Source, cfg.Database.AutoMigrate, log)
	defer entClient.Close()

	if err := bootstrap.EnsureDefaultApps(context.Background(), entClient, log, bootstrap.DefaultApps); err != nil {
		log.Fatal("初始化默认应用失败")
	}
}
