package shared

import (
	"context"

	"github.com/loqbit/ownforge/services/identity/internal/ent"
	"github.com/loqbit/ownforge/services/identity/internal/ent/app"
)

// EntClientFromCtx centralizes the logic for choosing tx inside a transaction and client outside one.
// In a transaction context, the returned *ent.Client is actually a client view bound to the current tx.
func EntClientFromCtx(ctx context.Context, fallback *ent.Client) *ent.Client {
	if tx := ent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}

// FindAppByCode looks up an app entity by app code.
func FindAppByCode(ctx context.Context, client *ent.Client, appCode string) (*ent.App, error) {
	return client.App.Query().
		Where(app.AppCodeEQ(appCode)).
		Only(ctx)
}
