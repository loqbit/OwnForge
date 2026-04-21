package prompt

import (
	"fmt"

	"github.com/ownforge/ownforge/services/notes/internal/platform/llm"
)

// PromptVersionEnrich 当前 enrich prompt 的版本号。
// 每次修改 SystemPromptEnrich 都要 bump 这个值，方便：
//   1. 识别存量数据是哪个 prompt 生成的
//   2. Prompt A/B 和回归评估
//   3. 灰度重算历史数据
const PromptVersionEnrich = "enrich-v1"

// PromptVersionWeeklyReport 周报 prompt 版本号。
const PromptVersionWeeklyReport = "weekly-v1"

// SystemPromptEnrich 文档增值的 System Prompt（固定内容，可缓存）。
const SystemPromptEnrich = `你是一个知识管理助手。用户会给你一篇文档和他已有的标签列表，请你完成以下三件事：

1. **标签**：建议 5-8 个分类标签供用户选择。**优先从用户已有的标签中匹配**，只有确实没有合适的才建议新标签。新标签用简洁的名称（类似 GitHub topics）
2. **待办**：提取文档中的待办事项（TODO / FIXME / 需要xxx / 待完成 等），如果没有就返回空数组
3. **摘要**：写一句话摘要（不超过 100 字），概括文档的核心内容

严格返回以下 JSON 格式，不要包含其他文字：
{
  "tags": ["标签1", "标签2", "标签3"],
  "todos": [
    {"text": "待办事项描述", "priority": "high"},
    {"text": "另一个待办", "priority": "medium"}
  ],
  "summary": "一句话摘要"
}

priority 取值：high / medium / low。如果无法判断优先级，默认 medium。`

// BuildEnrichMessages 构建文档增值的完整消息列表。
// existingTags 是用户已有的标签名称列表，AI 会优先从中匹配。
func BuildEnrichMessages(title, content string, existingTags []string) []llm.Message {
	tagsInfo := "无"
	if len(existingTags) > 0 {
		tagsInfo = fmt.Sprintf("%v", existingTags)
	}

	userContent := fmt.Sprintf("用户已有的标签：%s\n\n标题：%s\n\n内容：\n%s", tagsInfo, title, content)

	return []llm.Message{
		{Role: "system", Content: SystemPromptEnrich},
		{Role: "user", Content: userContent},
	}
}

// SystemPromptWeeklyReport 周报生成的 System Prompt。
const SystemPromptWeeklyReport = `你是一个生产力助手。用户会给你本周所有文档的摘要和标签信息。
请你基于这些信息，生成一份结构化的 Markdown 周报。

周报应包含：
1. **本周概览**：一段话总结本周的工作重点
2. **主要工作**：按主题/项目分组，列出具体做了什么
3. **待办跟进**：如果摘要中提到了遗留问题，列出来
4. **下周计划**：基于本周工作推断可能的下一步（简要即可）

使用 Markdown 格式，标题用 ##，列表用 -。语言简洁专业。`

// BuildWeeklyReportMessages 构建周报生成的消息列表。
func BuildWeeklyReportMessages(summaries string) []llm.Message {
	return []llm.Message{
		{Role: "system", Content: SystemPromptWeeklyReport},
		{Role: "user", Content: summaries},
	}
}
