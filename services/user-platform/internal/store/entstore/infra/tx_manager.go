package infrastore

import (
	"context"
	"fmt"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	infrarepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/infra"
)

type transactionManager struct {
	client *ent.Client
}

// NewTransactionManager 创建一个 TransactionManager 实例。
func NewTransactionManager(client *ent.Client) infrarepo.TransactionManager {
	return &transactionManager{client: client}
}

// WithTx 在同一个数据库事务中执行传入的业务函数。
func (tm *transactionManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting a transaction: %w", err)
	}

	// 捕获 panic 并确保安全回滚
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	txCtx := ent.NewTxContext(ctx, tx)

	// 执行业务逻辑
	if err := fn(txCtx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}

	// 顺利提交
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
