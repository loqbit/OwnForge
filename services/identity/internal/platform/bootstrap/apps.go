package bootstrap

import (
	"context"
	"fmt"

	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/ent/app"
	"go.uber.org/zap"
)

type SeedApp struct {
	Code string
	Name string
}

var DefaultApps = []SeedApp{
	{Code: "go-note", Name: "GoNote"},
	{Code: "go-chat", Name: "GoChat"},
}

func EnsureDefaultApps(ctx context.Context, client *ent.Client, log *zap.Logger, apps []SeedApp) error {
	for _, item := range apps {
		exists, err := client.App.Query().Where(app.AppCodeEQ(item.Code)).Exist(ctx)
		if err != nil {
			return fmt.Errorf("failed to query app %s: %w", item.Code, err)
		}
		if exists {
			continue
		}

		if _, err := client.App.Create().
			SetAppCode(item.Code).
			SetAppName(item.Name).
			Save(ctx); err != nil {
			if !ent.IsConstraintError(err) {
				return fmt.Errorf("failed to create app %s: %w", item.Code, err)
			}
		}

		log.Info("default apps initialized", zap.String("app_code", item.Code), zap.String("app_name", item.Name))
	}

	return nil
}
