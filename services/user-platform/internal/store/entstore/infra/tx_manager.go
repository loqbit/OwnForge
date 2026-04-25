package infrastore

import (
	"context"
	"fmt"

	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	infrarepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/infra"
)

type transactionManager struct {
	client *ent.Client
}

// NewTransactionManager creates a TransactionManager instance.
func NewTransactionManager(client *ent.Client) infrarepo.TransactionManager {
	return &transactionManager{client: client}
}

// WithTx executes the provided business function within a single database transaction.
func (tm *transactionManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting a transaction: %w", err)
	}

	// Catch panics and make sure the transaction rolls back safely.
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	txCtx := ent.NewTxContext(ctx, tx)

	// Run the business logic.
	if err := fn(txCtx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w: rolling back transaction: %v", err, rerr)
		}
		return err
	}

	// Commit on success.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
