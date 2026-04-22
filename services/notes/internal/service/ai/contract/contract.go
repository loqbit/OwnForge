package contract

// TodoItem is a structured todo item extracted by AI.
type TodoItem struct {
	Text     string `json:"text"`
	Priority string `json:"priority"` // "high" | "medium" | "low"
	Done     bool   `json:"done"`
}

// EnrichResult is the document-enrichment result, including tags, todos, and summary, produced by a single LLM call.
type EnrichResult struct {
	Summary  string     `json:"summary"`
	AutoTags []string   `json:"tags"`
	Todos    []TodoItem `json:"todos"`
}

// WeeklyReportResult is the result of weekly report generation.
type WeeklyReportResult struct {
	Title   string // "Weekly Report 2026-04-14 ~ 2026-04-20"
	Content string // weekly report body in Markdown format
}
