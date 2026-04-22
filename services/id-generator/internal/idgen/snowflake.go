package idgen

import (
	"context"
	"fmt"

	pkgid "github.com/ownforge/ownforge/pkg/id"
)

var generator pkgid.Generator

// Init initializes the in-process snowflake generator.
func Init(nodeID int64) error {
	gen, err := pkgid.NewLocalSnowflake(nodeID)
	if err != nil {
		return fmt.Errorf("初始化雪花算法节点失败: %w", err)
	}
	generator = gen
	return nil
}

// NextID returns the next ID from the configured generator.
func NextID(ctx context.Context) (int64, error) {
	if generator == nil {
		return 0, fmt.Errorf("雪花算法库未初始化，请先调用 Init(nodeID)")
	}
	return generator.NextID(ctx)
}
