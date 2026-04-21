package contract

// RecordCommand 记录文档来源信息。
type RecordCommand struct {
	SnippetID       int64
	SourceSnippetID *int64
	SourceShareID   *int64
	SourceUserID    *int64
	RelationType    string
}
