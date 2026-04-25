package idgen

import (
	"context"
	"fmt"

	pkgid "github.com/loqbit/ownforge/pkg/id"
)

var generator pkgid.Generator

// Init initializes the in-process snowflake generator.
func Init(nodeID int64) error {
	gen, err := pkgid.NewLocalSnowflake(nodeID)
	if err != nil {
		return fmt.Errorf("failed to initialize the Snowflake node: %w", err)
	}
	generator = gen
	return nil
}

// NextID returns the next ID from the configured generator.
func NextID(ctx context.Context) (int64, error) {
	if generator == nil {
		return 0, fmt.Errorf("Snowflake library not initialized; call Init(nodeID) first")
	}
	return generator.NextID(ctx)
}
