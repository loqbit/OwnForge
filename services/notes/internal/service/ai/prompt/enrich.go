package prompt

import (
	"fmt"

	"github.com/loqbit/ownforge/services/notes/internal/platform/llm"
)

// PromptVersionEnrich is the current version of the enrich prompt.
// Bump this value whenever SystemPromptEnrich changes so it is easier to:
//  1. identify which prompt produced existing data
//  2. run prompt A/B tests and regression evaluations
//  3. gradually recompute historical data
const PromptVersionEnrich = "enrich-v1"

// PromptVersionWeeklyReport is the version of the weekly report prompt.
const PromptVersionWeeklyReport = "weekly-v1"

// SystemPromptEnrich is the fixed, cacheable system prompt used for document enrichment.
const SystemPromptEnrich = `You are a knowledge management assistant. The user will provide a document and a list of their existing tags. Please complete the following three tasks:

1. **Tags**: Suggest 5-8 category tags for the user to choose from. **Prefer matching existing user tags first**. Only suggest new tags when there is truly no suitable existing tag. Keep new tag names concise, similar to GitHub topics.
2. **Todos**: Extract action items from the document (for example TODO, FIXME, "need to ...", "pending", etc.). Return an empty array if none are found.
3. **Summary**: Write a one-sentence summary in no more than 100 words that captures the core content of the document.

Return strictly the following JSON format and nothing else:
{
  "tags": ["tag1", "tag2", "tag3"],
  "todos": [
    {"text": "todo description", "priority": "high"},
    {"text": "another todo", "priority": "medium"}
  ],
  "summary": "one-sentence summary"
}

priority must be one of: high / medium / low. If priority cannot be determined, default to medium.`

// BuildEnrichMessages builds the full message list for document enrichment.
// existingTags is the list of the user's current tag names; the AI should prefer matching from it.
func BuildEnrichMessages(title, content string, existingTags []string) []llm.Message {
	tagsInfo := "none"
	if len(existingTags) > 0 {
		tagsInfo = fmt.Sprintf("%v", existingTags)
	}

	userContent := fmt.Sprintf("Existing user tags: %s\n\ntitle: %s\n\ncontent:\n%s", tagsInfo, title, content)

	return []llm.Message{
		{Role: "system", Content: SystemPromptEnrich},
		{Role: "user", Content: userContent},
	}
}

// SystemPromptWeeklyReport is the system prompt for weekly report generation.
const SystemPromptWeeklyReport = `You are a productivity assistant. The user will provide summaries and tag information for all documents created this week.
Based on this information, generate a structured Markdown weekly report.

The report should include:
1. **Weekly Overview**: A short paragraph summarizing the main focus of the week.
2. **Key Work**: Group concrete work items by topic or project.
3. **Todo Follow-up**: List unresolved items if they appear in the summaries.
4. **Next Week Plan**: Briefly infer likely next steps based on this week's work.

Use Markdown format with ## for headings and - for lists. Keep the language concise and professional.`

// BuildWeeklyReportMessages builds the message list for weekly report generation.
func BuildWeeklyReportMessages(summaries string) []llm.Message {
	return []llm.Message{
		{Role: "system", Content: SystemPromptWeeklyReport},
		{Role: "user", Content: summaries},
	}
}
