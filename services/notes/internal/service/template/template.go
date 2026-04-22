package template

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	templaterepo "github.com/ownforge/ownforge/services/notes/internal/repository/template"
	"github.com/ownforge/ownforge/services/notes/internal/service/template/contract"

	"go.uber.org/zap"
)

// Domain errors.
var (
	ErrIDGeneration   = pkgerrs.NewServerErr(errors.New("failed to generate template ID"))
	ErrForbidden      = pkgerrs.New(pkgerrs.Forbidden, "no permission to operate on this template", nil)
	ErrSystemReadOnly = pkgerrs.New(pkgerrs.Forbidden, "system templates cannot be modified or deleted", nil)
	ErrNameRequired   = pkgerrs.NewParamErr("template name cannot be empty", nil)
)

// TemplateService defines the template service interface.
type TemplateService interface {
	Create(ctx context.Context, userID int64, cmd *contract.CreateTemplateCommand) (*contract.TemplateResult, error)
	List(ctx context.Context, userID int64, category string) ([]contract.TemplateResult, error)
	GetByID(ctx context.Context, id int64) (*contract.TemplateResult, error)
	Update(ctx context.Context, userID, id int64, cmd *contract.UpdateTemplateCommand) (*contract.TemplateResult, error)
	Delete(ctx context.Context, userID, id int64) error
	SeedSystemTemplates(ctx context.Context) error
}

type templateService struct {
	repo   templaterepo.Repository
	idgen  idgen.Client
	logger *zap.Logger
}

// NewTemplateService creates a TemplateService instance.
func NewTemplateService(repo templaterepo.Repository, idgenClient idgen.Client, logger *zap.Logger) TemplateService {
	return &templateService{repo: repo, idgen: idgenClient, logger: logger}
}

func (s *templateService) Create(ctx context.Context, userID int64, cmd *contract.CreateTemplateCommand) (*contract.TemplateResult, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate template ID", zap.Error(err))
		return nil, ErrIDGeneration
	}

	params := &contract.CreateTemplateCommand{
		Name:        name,
		Description: strings.TrimSpace(cmd.Description),
		Content:     cmd.Content,
		Language:    normalizeLanguage(cmd.Language),
		Category:    normalizeCategory(cmd.Category),
	}

	t, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to create template", zap.Int64("id", id), zap.Error(err))
		return nil, err
	}

	return toResult(t), nil
}

func (s *templateService) List(ctx context.Context, userID int64, category string) ([]contract.TemplateResult, error) {
	list, err := s.repo.List(ctx, userID, category)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to query templates", zap.Int64("userID", userID), zap.Error(err))
		return nil, err
	}

	results := make([]contract.TemplateResult, 0, len(list))
	for _, item := range list {
		results = append(results, *toResult(&item))
	}

	return results, nil
}

func (s *templateService) GetByID(ctx context.Context, id int64) (*contract.TemplateResult, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResult(t), nil
}

func (s *templateService) Update(ctx context.Context, userID, id int64, cmd *contract.UpdateTemplateCommand) (*contract.TemplateResult, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// System templates cannot be modified.
	if current.IsSystem {
		return nil, ErrSystemReadOnly
	}

	// Only your own templates can be modified.
	if current.OwnerID != userID {
		return nil, ErrForbidden
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	params := &contract.UpdateTemplateCommand{
		Name:        name,
		Description: strings.TrimSpace(cmd.Description),
		Content:     cmd.Content,
		Language:    normalizeLanguage(cmd.Language),
		Category:    normalizeCategory(cmd.Category),
	}

	t, err := s.repo.Update(ctx, userID, id, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to update template", zap.Int64("id", id), zap.Error(err))
		return nil, err
	}

	return toResult(t), nil
}

func (s *templateService) Delete(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// System templates cannot be deleted.
	if current.IsSystem {
		return ErrSystemReadOnly
	}

	// Only your own templates can be deleted.
	if current.OwnerID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to delete template", zap.Int64("id", id), zap.Error(err))
		return err
	}

	return nil
}

// SeedSystemTemplates initializes built-in system templates. The operation is idempotent and skips existing entries.
func (s *templateService) SeedSystemTemplates(ctx context.Context) error {
	count, err := s.repo.CountSystem(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		s.logger.Info("system templates already exist, skipping seed", zap.Int("count", count))
		return nil
	}

	// Generate 3 Snowflake IDs.
	ids := make([]int64, 3)
	for i := range ids {
		id, err := s.idgen.NextID(ctx)
		if err != nil {
			return ErrIDGeneration
		}
		ids[i] = id
	}

	templates := []templaterepo.Template{
		{
			ID:          ids[0],
			OwnerID:     0,
			Name:        "Meeting Notes",
			Description: "Standard meeting notes template with date, attendees, agenda, decisions, and action items.",
			Content:     seedMeetingNotes,
			Language:    "markdown",
			Category:    "meeting",
			IsSystem:    true,
		},
		{
			ID:          ids[1],
			OwnerID:     0,
			Name:        "Technical Design",
			Description: "Technical design document template for new features, architectural refactors, and similar decisions.",
			Content:     seedTechDesign,
			Language:    "markdown",
			Category:    "tech_design",
			IsSystem:    true,
		},
		{
			ID:          ids[2],
			OwnerID:     0,
			Name:        "Weekly Report",
			Description: "Weekly report template for tracking this week's progress, next week's plan, and support needed.",
			Content:     seedWeeklyReport,
			Language:    "markdown",
			Category:    "weekly_report",
			IsSystem:    true,
		},
	}

	if err := s.repo.CreateBatch(ctx, templates); err != nil {
		s.logger.Error("failed to seed system templates", zap.Error(err))
		return err
	}

	s.logger.Info("system templates initialized", zap.Int("count", len(templates)))
	return nil
}

func toResult(t *templaterepo.Template) *contract.TemplateResult {
	if t == nil {
		return nil
	}

	return &contract.TemplateResult{
		ID:          t.ID,
		OwnerID:     t.OwnerID,
		Name:        t.Name,
		Description: t.Description,
		Content:     t.Content,
		Language:    t.Language,
		Category:    t.Category,
		IsSystem:    t.IsSystem,
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
	}
}

func normalizeLanguage(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "markdown"
	}
	return lang
}

func normalizeCategory(cat string) string {
	cat = strings.TrimSpace(cat)
	if cat == "" {
		return "general"
	}
	return cat
}

// Seed template contents

const seedMeetingNotes = `# Meeting Notes

## Basic Information

- **Date**: YYYY-MM-DD
- **Time**: HH:MM - HH:MM
- **Location**:
- **Facilitator**:
- **Attendees**:

## Agenda

1. 
2. 
3. 

## Discussion Notes

### Topic 1

- 

### Topic 2

- 

## Decisions

- [ ] 
- [ ] 

## Action Items

| No. | Item | Owner | Due Date | Status |
|------|------|--------|----------|------|
| 1    |      |        |          | Pending |
| 2    |      |        |          | Pending |

## Next Meeting

- **Date**:
- **Planned Agenda**:
`

const seedTechDesign = `# Technical Design

## 1. Background and Goals

### 1.1 Background

Describe the origin of the current problem or requirement.

### 1.2 Goals

- 
- 

### 1.3 Non-goals

- 

## 2. Solution Design

### 2.1 Overall Architecture

Describe the overall design approach for the solution.

### 2.2 Detailed Design

#### Data Model

#### Interface Design

#### Core Flow

### 2.3 Technology Choices

| Component | Choice | Reason |
|------|------|------|
|      |      |      |

## 3. Milestones

| Phase | Scope | Estimated Time |
|------|------|----------|
| P0   |      |          |
| P1   |      |          |

## 4. Risk Assessment

| Risk | Impact | Mitigation |
|------|------|----------|
|      |      |          |

## 5. References

- 
`

const seedWeeklyReport = `# Weekly Report

**Name**:
**Period**: YYYY-MM-DD ~ YYYY-MM-DD

## Completed This Week

- [ ] 
- [ ] 
- [ ] 

## Plan for Next Week

- [ ] 
- [ ] 

## Support Needed

- 

## Summary and Reflection

This week mainly focused on...

## Metrics (Optional)

| Metric | Last Week | This Week | Change |
|------|------|------|------|
|      |      |      |      |
`
