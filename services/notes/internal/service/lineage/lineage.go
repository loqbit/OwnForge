package lineage

import (
	"context"
	"errors"
	"strings"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	lineagerepo "github.com/ownforge/ownforge/services/notes/internal/repository/lineage"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"github.com/ownforge/ownforge/services/notes/internal/service/lineage/contract"
	"go.uber.org/zap"
)

var (
	ErrLineageIDGeneration = pkgerrs.NewServerErr(errors.New("failed to generate lineage record ID"))
	ErrInvalidRelationType = pkgerrs.NewParamErr("invalid relation_type", nil)
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

	// Idempotent: skip creation when the record already exists.
	// Treat "not found" as normal and continue, but bubble up real database failures.
	existing, err := s.repo.GetBySnippetID(ctx, cmd.SnippetID)
	if err == nil && existing != nil {
		return nil // Record already exists; skip for idempotency.
	}
	if err != nil && !sharedrepo.IsNotFoundError(err) {
		commonlogger.Ctx(ctx, s.logger).Error("failed to query lineage", zap.Int64("snippet_id", cmd.SnippetID), zap.Error(err))
		return err
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate lineage ID", zap.Error(err))
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
