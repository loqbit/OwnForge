package idgen

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

var node *snowflake.Node

// Init 初始化雪花算法节点
// nodeID 范围是 0 到 1023（取决于 Snowflake 实现，bwmarrin 默认是 10-bit Node ID）
func Init(nodeID int64) error {
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("初始化雪花算法节点失败: %w", err)
	}
	node = n
	return nil
}

// NextID 生成下一个唯一 ID
func NextID() int64 {
	if node == nil {
		panic("雪花算法库未初始化，请先调用 Init(nodeID)")
	}
	return node.Generate().Int64()
}
