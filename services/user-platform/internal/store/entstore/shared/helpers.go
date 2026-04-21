package shared

import (
	"context"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	"github.com/ownforge/ownforge/services/user-platform/internal/ent/app"
)

// EntClientFromCtx 统一处理“事务中用 tx、非事务用 client”的判断逻辑。
// 返回的 *ent.Client 在事务上下文中实际上是一个绑定到当前 tx 的 client 视图。
func EntClientFromCtx(ctx context.Context, fallback *ent.Client) *ent.Client {
	if tx := ent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}

// FindAppByCode 按应用编码查询应用实体。
func FindAppByCode(ctx context.Context, client *ent.Client, appCode string) (*ent.App, error) {
	return client.App.Query().
		Where(app.AppCodeEQ(appCode)).
		Only(ctx)
}
