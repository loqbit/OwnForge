package contract

// RecordCommand records document lineage information.
type RecordCommand struct {
	SnippetID       int64
	SourceSnippetID *int64
	SourceShareID   *int64
	SourceUserID    *int64
	RelationType    string
}
