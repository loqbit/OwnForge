package aicallog

import "context"

// Repository 定义 AI 调用日志的数据访问接口。
//
// 设计原则：Insert 只追加、永不更新。查询主要用于账单、Dashboard、排障。
type Repository interface {
	// Insert 落一条调用记录。Entry.ID 由调用方通过 id-generator 提供。
	Insert(ctx context.Context, entry *Entry) error
}
