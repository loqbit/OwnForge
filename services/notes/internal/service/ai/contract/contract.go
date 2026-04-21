package contract

// TodoItem AI 提取的结构化待办项。
type TodoItem struct {
	Text     string `json:"text"`
	Priority string `json:"priority"` // "high" | "medium" | "low"
	Done     bool   `json:"done"`
}

// EnrichResult 文档增值的结果（标签 + TODO + 摘要，一次 LLM 调用完成）。
type EnrichResult struct {
	Summary  string     `json:"summary"`
	AutoTags []string   `json:"tags"`
	Todos    []TodoItem `json:"todos"`
}

// WeeklyReportResult 周报生成结果。
type WeeklyReportResult struct {
	Title   string // "周报 2026-04-14 ~ 2026-04-20"
	Content string // Markdown 格式的周报正文
}
