package template

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/luckysxx/common/errs"
	commonlogger "github.com/luckysxx/common/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	templaterepo "github.com/ownforge/ownforge/services/notes/internal/repository/template"
	"github.com/ownforge/ownforge/services/notes/internal/service/template/contract"

	"go.uber.org/zap"
)

// 领域错误定义
var (
	ErrIDGeneration   = pkgerrs.NewServerErr(errors.New("生成模板 ID 失败"))
	ErrForbidden      = pkgerrs.New(pkgerrs.Forbidden, "无权限操作该模板", nil)
	ErrSystemReadOnly = pkgerrs.New(pkgerrs.Forbidden, "系统模板不可修改或删除", nil)
	ErrNameRequired   = pkgerrs.NewParamErr("模板名称不能为空", nil)
)

// TemplateService 定义 template 业务接口。
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

// NewTemplateService 创建 TemplateService 实例。
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
		commonlogger.Ctx(ctx, s.logger).Error("生成 template ID 失败", zap.Error(err))
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
		commonlogger.Ctx(ctx, s.logger).Error("创建 template 失败", zap.Int64("id", id), zap.Error(err))
		return nil, err
	}

	return toResult(t), nil
}

func (s *templateService) List(ctx context.Context, userID int64, category string) ([]contract.TemplateResult, error) {
	list, err := s.repo.List(ctx, userID, category)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("查询模板列表失败", zap.Int64("userID", userID), zap.Error(err))
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

	// 系统模板不可修改
	if current.IsSystem {
		return nil, ErrSystemReadOnly
	}

	// 只能修改自己的模板
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
		commonlogger.Ctx(ctx, s.logger).Error("更新 template 失败", zap.Int64("id", id), zap.Error(err))
		return nil, err
	}

	return toResult(t), nil
}

func (s *templateService) Delete(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 系统模板不可删除
	if current.IsSystem {
		return ErrSystemReadOnly
	}

	// 只能删除自己的模板
	if current.OwnerID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("删除 template 失败", zap.Int64("id", id), zap.Error(err))
		return err
	}

	return nil
}

// SeedSystemTemplates 初始化系统预置模板（幂等：已存在则跳过）。
func (s *templateService) SeedSystemTemplates(ctx context.Context) error {
	count, err := s.repo.CountSystem(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		s.logger.Info("系统模板已存在，跳过 seed", zap.Int("count", count))
		return nil
	}

	// 生成 3 个雪花 ID
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
			Name:        "会议记录",
			Description: "标准会议纪要模板，包含日期、参会人、议题、决议和待办事项。",
			Content:     seedMeetingNotes,
			Language:    "markdown",
			Category:    "meeting",
			IsSystem:    true,
		},
		{
			ID:          ids[1],
			OwnerID:     0,
			Name:        "技术方案",
			Description: "技术设计文档模板，适用于新功能设计、架构重构等技术决策。",
			Content:     seedTechDesign,
			Language:    "markdown",
			Category:    "tech_design",
			IsSystem:    true,
		},
		{
			ID:          ids[2],
			OwnerID:     0,
			Name:        "周报",
			Description: "周报模板，记录本周进展、下周计划和需要支持的事项。",
			Content:     seedWeeklyReport,
			Language:    "markdown",
			Category:    "weekly_report",
			IsSystem:    true,
		},
	}

	if err := s.repo.CreateBatch(ctx, templates); err != nil {
		s.logger.Error("seed 系统模板失败", zap.Error(err))
		return err
	}

	s.logger.Info("系统模板初始化完成", zap.Int("count", len(templates)))
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

// ── Seed 模板内容 ──────────────────────────────────────────────

const seedMeetingNotes = `# 会议记录

## 基本信息

- **日期**：YYYY-MM-DD
- **时间**：HH:MM - HH:MM
- **地点**：
- **主持人**：
- **参会人**：

## 会议议题

1. 
2. 
3. 

## 讨论记录

### 议题一

- 

### 议题二

- 

## 决议

- [ ] 
- [ ] 

## Action Items

| 序号 | 事项 | 负责人 | 截止日期 | 状态 |
|------|------|--------|----------|------|
| 1    |      |        |          | 待完成 |
| 2    |      |        |          | 待完成 |

## 下次会议

- **日期**：
- **议题预告**：
`

const seedTechDesign = `# 技术方案

## 1. 背景与目标

### 1.1 背景

描述当前问题或需求的来源。

### 1.2 目标

- 
- 

### 1.3 非目标

- 

## 2. 方案设计

### 2.1 整体架构

描述方案的整体设计思路。

### 2.2 详细设计

#### 数据模型

#### 接口设计

#### 核心流程

### 2.3 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
|      |      |      |

## 3. 里程碑

| 阶段 | 内容 | 预计时间 |
|------|------|----------|
| P0   |      |          |
| P1   |      |          |

## 4. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
|      |      |          |

## 5. 参考资料

- 
`

const seedWeeklyReport = `# 周报

**姓名**：
**周期**：YYYY-MM-DD ~ YYYY-MM-DD

## 本周完成

- [ ] 
- [ ] 
- [ ] 

## 下周计划

- [ ] 
- [ ] 

## 需要协助

- 

## 总结与思考

本周主要...

## 数据指标（可选）

| 指标 | 上周 | 本周 | 变化 |
|------|------|------|------|
|      |      |      |      |
`
