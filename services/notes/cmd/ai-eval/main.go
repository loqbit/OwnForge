// Command ai-eval runs the Enrich golden-set regression.
//
// Usage:
//
//	go run ./cmd/ai-eval                        # Run all cases
//	go run ./cmd/ai-eval -case case_01_meeting  # Run only the specified case
//	go run ./cmd/ai-eval -dir testdata/eval/enrich -verbose
//
// Exit codes: 0 = all passed; 1 = one or more cases failed; 2 = runtime error (config/network).
//
// Does not connect to the DB or publish events; only tests the core prompt -> LLM -> parse path.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/ownforge/ownforge/services/notes/internal/platform/config"
	"github.com/ownforge/ownforge/services/notes/internal/platform/llm"
	ai "github.com/ownforge/ownforge/services/notes/internal/service/ai"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/contract"
	"github.com/ownforge/ownforge/services/notes/internal/service/ai/prompt"
)

// ── Case definitions ──────────────────────────────────────────────────

type Case struct {
	Name  string `json:"name"`
	Input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	} `json:"input"`
	Expect Expect `json:"expect"`
}

type Expect struct {
	Tags    *TagsExpect    `json:"tags,omitempty"`
	Todos   *TodosExpect   `json:"todos,omitempty"`
	Summary *SummaryExpect `json:"summary,omitempty"`
}

type TagsExpect struct {
	CountRange       []int      `json:"count_range,omitempty"`         // [min, max]
	MustIncludeAnyOf [][]string `json:"must_include_any_of,omitempty"` // Match any item within each group; all groups must match (AND)
	MustNotInclude   []string   `json:"must_not_include,omitempty"`
}

type TodosExpect struct {
	MinCount       int      `json:"min_count,omitempty"`
	MaxCount       int      `json:"max_count,omitempty"`
	MustMentionAny []string `json:"must_mention_any,omitempty"` // The TODO text must mention at least one of these
}

type SummaryExpect struct {
	MaxChars       int      `json:"max_chars,omitempty"`
	MinChars       int      `json:"min_chars,omitempty"`
	MustMentionAny []string `json:"must_mention_any,omitempty"`
}

// ── Validation logic ──────────────────────────────────────────────────

type Violation struct {
	Dim    string // "tags" / "todos" / "summary"
	Reason string
}

func check(r *contract.EnrichResult, e Expect) []Violation {
	var vs []Violation
	vs = append(vs, checkTags(r.AutoTags, e.Tags)...)
	vs = append(vs, checkTodos(r.Todos, e.Todos)...)
	vs = append(vs, checkSummary(r.Summary, e.Summary)...)
	return vs
}

func checkTags(got []string, exp *TagsExpect) []Violation {
	if exp == nil {
		return nil
	}
	var vs []Violation
	lower := toLower(got)

	if len(exp.CountRange) == 2 {
		if len(got) < exp.CountRange[0] || len(got) > exp.CountRange[1] {
			vs = append(vs, Violation{"tags",
				fmt.Sprintf("count expected in [%d,%d] got=%d (tags=%v)", exp.CountRange[0], exp.CountRange[1], len(got), got)})
		}
	}

	for _, group := range exp.MustIncludeAnyOf {
		if !anyContainedIn(lower, group) {
			vs = append(vs, Violation{"tags",
				fmt.Sprintf("must include any of %v (tags=%v)", group, got)})
		}
	}

	for _, bad := range exp.MustNotInclude {
		if containsIgnoreCase(lower, bad) {
			vs = append(vs, Violation{"tags",
				fmt.Sprintf("must not include %q (tags=%v)", bad, got)})
		}
	}
	return vs
}

func checkTodos(got []contract.TodoItem, exp *TodosExpect) []Violation {
	if exp == nil {
		return nil
	}
	var vs []Violation
	if exp.MinCount > 0 && len(got) < exp.MinCount {
		vs = append(vs, Violation{"todos", fmt.Sprintf("min_count expected>=%d got=%d", exp.MinCount, len(got))})
	}
	if exp.MaxCount > 0 && len(got) > exp.MaxCount {
		vs = append(vs, Violation{"todos", fmt.Sprintf("max_count expected<=%d got=%d", exp.MaxCount, len(got))})
	}
	if len(exp.MustMentionAny) > 0 {
		joined := strings.ToLower(joinTodos(got))
		hit := false
		for _, kw := range exp.MustMentionAny {
			if strings.Contains(joined, strings.ToLower(kw)) {
				hit = true
				break
			}
		}
		if !hit {
			vs = append(vs, Violation{"todos",
				fmt.Sprintf("must mention any of %v (todos=%v)", exp.MustMentionAny, todoTexts(got))})
		}
	}
	return vs
}

func checkSummary(got string, exp *SummaryExpect) []Violation {
	if exp == nil {
		return nil
	}
	var vs []Violation
	n := len([]rune(got))
	if exp.MaxChars > 0 && n > exp.MaxChars {
		vs = append(vs, Violation{"summary", fmt.Sprintf("len expected<=%d got=%d", exp.MaxChars, n)})
	}
	if exp.MinChars > 0 && n < exp.MinChars {
		vs = append(vs, Violation{"summary", fmt.Sprintf("len expected>=%d got=%d", exp.MinChars, n)})
	}
	if len(exp.MustMentionAny) > 0 {
		lower := strings.ToLower(got)
		hit := false
		for _, kw := range exp.MustMentionAny {
			if strings.Contains(lower, strings.ToLower(kw)) {
				hit = true
				break
			}
		}
		if !hit {
			vs = append(vs, Violation{"summary",
				fmt.Sprintf("must mention any of %v (summary=%q)", exp.MustMentionAny, got)})
		}
	}
	return vs
}

// ── Main flow ─────────────────────────────────────────────────────────

func main() {
	_ = godotenv.Load()

	var (
		dir     = flag.String("dir", "testdata/eval/enrich", "Golden set directory")
		only    = flag.String("case", "", "Run only the specified case name (without .json)")
		verbose = flag.Bool("verbose", false, "Print the full output for each case")
	)
	flag.Parse()

	cfg := config.LoadConfig()
	client := newLLMClient(cfg.AI)

	cases, err := loadCases(*dir, *only)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load cases: %v\n", err)
		os.Exit(2)
	}
	if len(cases) == 0 {
		fmt.Fprintf(os.Stderr, "no cases found in %s\n", *dir)
		os.Exit(2)
	}

	fmt.Printf("Eval: enrich\n")
	fmt.Printf("  Provider: %s  Model: %s\n", cfg.AI.Provider, cfg.AI.EnrichModel)
	fmt.Printf("  Cases: %d  Dir: %s\n\n", len(cases), *dir)

	ctx := context.Background()
	var (
		pass, fail int
		totalIn    int
		totalOut   int
		totalMS    int
	)
	started := time.Now()

	for _, c := range cases {
		result, stats, err := ai.EnrichOnce(ctx, client, cfg.AI.EnrichModel, cfg.AI.MaxTokens, c.Input.Title, c.Input.Content, nil)
		if err != nil {
			fail++
			fmt.Printf("💥 %s  [ERROR] %v\n", c.Name, err)
			continue
		}
		totalIn += stats.InputTokens
		totalOut += stats.OutputTokens
		totalMS += stats.LatencyMS

		vs := check(result, c.Expect)
		if len(vs) == 0 {
			pass++
			fmt.Printf("✅ %-40s  in=%d out=%d %dms\n", c.Name, stats.InputTokens, stats.OutputTokens, stats.LatencyMS)
			if *verbose {
				printResult(result)
			}
		} else {
			fail++
			fmt.Printf("❌ %-40s  in=%d out=%d %dms\n", c.Name, stats.InputTokens, stats.OutputTokens, stats.LatencyMS)
			for _, v := range vs {
				fmt.Printf("     · [%s] %s\n", v.Dim, v.Reason)
			}
			if *verbose {
				printResult(result)
			}
		}
	}

	total := pass + fail
	rate := 0.0
	if total > 0 {
		rate = float64(pass) * 100 / float64(total)
	}
	fmt.Printf("\n────────────────────────────────────────────\n")
	fmt.Printf("Pass rate: %d/%d  (%.1f%%)\n", pass, total, rate)
	if total > 0 {
		fmt.Printf("Average tokens: in=%d out=%d\n", totalIn/total, totalOut/total)
		fmt.Printf("Average latency: %dms\n", totalMS/total)
	}
	fmt.Printf("Prompt version: %s\n", prompt.PromptVersionEnrich)
	fmt.Printf("Total duration: %s\n", time.Since(started).Round(time.Millisecond))

	if fail > 0 {
		os.Exit(1)
	}
}

// ── Helper functions ──────────────────────────────────────────────────

func loadCases(dir, only string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if only != "" && name != only {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)

	cases := make([]Case, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		if c.Name == "" {
			c.Name = strings.TrimSuffix(filepath.Base(f), ".json")
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func newLLMClient(cfg config.AIConfig) llm.Client {
	switch cfg.Provider {
	case "anthropic":
		return llm.NewAnthropicClient(cfg.BaseURL, cfg.APIKey)
	default:
		return llm.NewOpenAICompatClient(cfg.BaseURL, cfg.APIKey)
	}
}

func toLower(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
}

func containsIgnoreCase(haystack []string, needle string) bool {
	n := strings.ToLower(needle)
	for _, h := range haystack {
		if strings.Contains(h, n) {
			return true
		}
	}
	return false
}

func anyContainedIn(haystack, needles []string) bool {
	for _, n := range needles {
		if containsIgnoreCase(haystack, n) {
			return true
		}
	}
	return false
}

func joinTodos(todos []contract.TodoItem) string {
	parts := make([]string, len(todos))
	for i, t := range todos {
		parts[i] = t.Text
	}
	return strings.Join(parts, " | ")
}

func todoTexts(todos []contract.TodoItem) []string {
	out := make([]string, len(todos))
	for i, t := range todos {
		out[i] = t.Text
	}
	return out
}

func printResult(r *contract.EnrichResult) {
	fmt.Printf("     tags    : %v\n", r.AutoTags)
	fmt.Printf("     todos   : %v\n", todoTexts(r.Todos))
	fmt.Printf("     summary : %s\n", r.Summary)
}
