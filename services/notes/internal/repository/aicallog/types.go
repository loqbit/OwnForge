package aicallog

import "time"

// CallStatus is the result status of an AI call.
type CallStatus string

const (
	StatusSuccess    CallStatus = "success"
	StatusParseError CallStatus = "parse_error"
	StatusLLMError   CallStatus = "llm_error"
	StatusSkipped    CallStatus = "skipped"
	StatusTimeout    CallStatus = "timeout"
)

// Entry is one AI call trace record.
type Entry struct {
	ID            int64
	OwnerID       int64
	Skill         string
	SnippetID     *int64
	Provider      string
	Model         string
	PromptVersion string
	InputHash     string
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	CostUSD       float64
	LatencyMS     int
	Status        CallStatus
	Error         string
	CreatedAt     time.Time
}
