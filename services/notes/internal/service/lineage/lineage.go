package lineage

import (
	"context"
	"errors"
	"strings"

	pkgerrs "github.com/luckysxx/common/errs"
	commonlogger "github.com/luckysxx/common/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	lineagerepo "github.com/ownforge/ownforge/services/notes/internal/repository/lineage"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"github.com/ownforge/ownforge/services/notes/internal/service/lineage/contract"
	"go.uber.org/zap"
)

var (
	ErrLineageIDGeneration = pkgerrs.NewServerErr(errors.New("生成来源记录 ID 失败"))
	ErrInvalidRelationType = pkgerrs.NewParamErr("relation_type 不合法", nil)
)

type Service interface {
	Record(ctx context.Context, cmd *contract.RecordCommand) error
}

type lineageService struct {
	repo   lineagerepo.Repository
	idgen  idgen.Client
	logger *zap.Logger
}

func NewService(repo lineagerepo.Repository, idgenClient idgen.Client, logger *zap.Logger) Service {
	return &lineageService{repo: repo, idgen: idgenClient, logger: logger}
}

func (s *lineageService) Record(ctx context.Context, cmd *contract.RecordCommand) error {
	if cmd == nil || cmd.SnippetID <= 0 {
		return nil
	}

	relationType := normalizeRelationType(cmd.RelationType)
	if relationType == "" {
		return ErrInvalidRelationType
	}

	// 幂等：已存在就跳过。
	// 注意要区分 "不存在"（正常 → 继续创建）和 "数据库故障"（异常 → 上抛）。
	existing, err := s.repo.GetBySnippetID(ctx, cmd.SnippetID)
	if err == nil && existing != nil {
		return nil // 已有记录，幂等跳过
	}
	if err != nil && !sharedrepo.IsNotFoundError(err) {
		commonlogger.Ctx(ctx, s.logger).Error("查询 lineage 失败", zap.Int64("snippet_id", cmd.SnippetID), zap.Error(err))
		return err
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("生成 lineage ID 失败", zap.Error(err))
		return ErrLineageIDGeneration
	}

	_, err = s.repo.Create(ctx, &lineagerepo.Lineage{
		ID:              id,
		SnippetID:       cmd.SnippetID,
		SourceSnippetID: cmd.SourceSnippetID,
		SourceShareID:   cmd.SourceShareID,
		SourceUserID:    cmd.SourceUserID,
		RelationType:    relationType,
	})
	return err
}

func normalizeRelationType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "fork":
		return "fork"
	case "duplicate":
		return "duplicate"
	case "template":
		return "template"
	case "", "import":
		return "import"
	default:
		return ""
	}
}
