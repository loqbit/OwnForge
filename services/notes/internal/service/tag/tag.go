package tag

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/tag/contract"

	"go.uber.org/zap"
)

// 领域错误定义
var (
	ErrIDGeneration = pkgerrs.NewServerErr(errors.New("生成标签 ID 失败"))
	ErrForbidden    = pkgerrs.New(pkgerrs.Forbidden, "无权限操作该标签", nil)
	ErrNameRequired = pkgerrs.NewParamErr("标签名称不能为空", nil)
)

// TagService 定义 tag 业务接口。
type TagService interface {
	Create(ctx context.Context, userID int64, req *contract.CreateTagCommand) (*contract.TagResult, error)
	List(ctx context.Context, userID int64) ([]contract.TagResult, error)
	Update(ctx context.Context, userID, id int64, req *contract.UpdateTagCommand) (*contract.TagResult, error)
	Delete(ctx context.Context, userID, id int64) error
}

type tagService struct {
	repo   tagrepo.Repository
	idgen  idgen.Client
	logger *zap.Logger
}

// NewTagService 创建 TagService 实例。
func NewTagService(repo tagrepo.Repository, idgenClient idgen.Client, logger *zap.Logger) TagService {
	return &tagService{repo: repo, idgen: idgenClient, logger: logger}
}

func (s *tagService) Create(ctx context.Context, userID int64, req *contract.CreateTagCommand) (*contract.TagResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("生成 tag ID 失败", zap.Error(err))
		return nil, ErrIDGeneration
	}

	params := &contract.CreateTagCommand{
		Name:  name,
		Color: normalizeColor(req.Color),
	}

	t, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("创建 tag 失败",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return toTagResult(t), nil
}

func (s *tagService) List(ctx context.Context, userID int64) ([]contract.TagResult, error) {
	list, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("查询用户 tag 列表失败",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	results := make([]contract.TagResult, 0, len(list))
	for _, item := range list {
		results = append(results, *toTagResult(&item))
	}

	return results, nil
}

func (s *tagService) Update(ctx context.Context, userID, id int64, req *contract.UpdateTagCommand) (*contract.TagResult, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if current.OwnerID != userID {
		return nil, ErrForbidden
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	params := &contract.UpdateTagCommand{
		Name:  name,
		Color: normalizeColor(req.Color),
	}

	t, err := s.repo.Update(ctx, userID, id, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("更新 tag 失败",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return nil, err
	}

	return toTagResult(t), nil
}

func (s *tagService) Delete(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if current.OwnerID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("删除 tag 失败",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return err
	}

	return nil
}

func toTagResult(t *tagrepo.Tag) *contract.TagResult {
	if t == nil {
		return nil
	}

	return &contract.TagResult{
		ID:        t.ID,
		OwnerID:   t.OwnerID,
		Name:      t.Name,
		Color:     t.Color,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
}

func normalizeColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "#6366f1"
	}
	return c
}
