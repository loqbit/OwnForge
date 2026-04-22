package group

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	commonlogger "github.com/ownforge/ownforge/pkg/logger"
	"github.com/ownforge/ownforge/services/notes/internal/platform/idgen"
	grouprepo "github.com/ownforge/ownforge/services/notes/internal/repository/group"
	"github.com/ownforge/ownforge/services/notes/internal/service/group/contract"

	"go.uber.org/zap"
)

// 领域错误定义
var (
	ErrIDGeneration  = pkgerrs.NewServerErr(errors.New("生成分组 ID 失败"))
	ErrForbidden     = pkgerrs.New(pkgerrs.Forbidden, "无权限操作该分组", nil)
	ErrHasChildren   = pkgerrs.NewParamErr("分组下存在子分组，请先删除或移动", nil)
	ErrHasSnippets   = pkgerrs.NewParamErr("分组下存在文档，请先删除或移动", nil)
	ErrNameRequired  = pkgerrs.NewParamErr("分组名称不能为空", nil)
	ErrCycleDetected = pkgerrs.NewParamErr("检测到循环引用：不能将分组移动到自己的子孙节点下", nil)
)

// GroupService 定义 group 业务接口。
type GroupService interface {
	Create(ctx context.Context, userID int64, req *contract.CreateGroupCommand) (*contract.GroupResult, error)
	GetByID(ctx context.Context, userID, id int64) (*contract.GroupResult, error)
	List(ctx context.Context, userID int64, parentID *int64) ([]contract.GroupResult, error)
	Update(ctx context.Context, userID, id int64, req *contract.UpdateGroupCommand) (*contract.GroupResult, error)
	Delete(ctx context.Context, userID, id int64) error
	GetTree(ctx context.Context, userID int64) ([]contract.GroupTreeNode, error)
}

type groupService struct {
	repo   grouprepo.Repository
	idgen  idgen.Client
	logger *zap.Logger
}

// NewGroupService 创建 GroupService 实例。
func NewGroupService(repo grouprepo.Repository, idgenClient idgen.Client, logger *zap.Logger) GroupService {
	return &groupService{repo: repo, idgen: idgenClient, logger: logger}
}

func (s *groupService) Create(ctx context.Context, userID int64, req *contract.CreateGroupCommand) (*contract.GroupResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("生成 group ID 失败", zap.Error(err))
		return nil, ErrIDGeneration
	}

	params := &contract.CreateGroupCommand{
		ParentID:    req.ParentID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}

	g, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("创建 group 失败",
			zap.Int64("id", id),
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return s.enrichResult(ctx, g), nil
}

func (s *groupService) GetByID(ctx context.Context, userID, id int64) (*contract.GroupResult, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if g.OwnerID != userID {
		return nil, ErrForbidden
	}

	return s.enrichResult(ctx, g), nil
}

func (s *groupService) List(ctx context.Context, userID int64, parentID *int64) ([]contract.GroupResult, error) {
	list, err := s.repo.ListByOwner(ctx, userID, parentID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("查询用户 group 列表失败",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	results := make([]contract.GroupResult, 0, len(list))
	for _, item := range list {
		results = append(results, *s.enrichResult(ctx, &item))
	}

	return results, nil
}

func (s *groupService) Update(ctx context.Context, userID, id int64, req *contract.UpdateGroupCommand) (*contract.GroupResult, error) {
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

	params := &contract.UpdateGroupCommand{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		SortOrder:   req.SortOrder,
		ParentID:    req.ParentID,
	}

	// ─── 技术点 2：防循环引用 ────────────────────────────────────
	// 场景：把 A 移到 A 的子孙 C 下面 → A → C → ... → A 形成环。
	// 算法：从目标 parent 开始，沿 parent_id 链向上遍历祖先，
	//        如果走到了 id 本身 → 说明形成环 → 拒绝。
	//        如果走到 nil（顶级）→ 安全。
	// 复杂度：O(depth)，最坏 O(n)，但实际目录深度很少超过 10 层。
	if params.ParentID != nil {
		newParentID := *params.ParentID

		// 不能把自己移到自己下面
		if newParentID == id {
			return nil, ErrCycleDetected
		}

		// 需要沿祖先链检查：一次查全部 → 建 parentMap → 向上爬
		allGroups, err := s.repo.ListAllByOwner(ctx, userID)
		if err != nil {
			return nil, err
		}

		parentMap := make(map[int64]int64) //  child → parent
		for _, g := range allGroups {
			if g.ParentID != nil {
				parentMap[g.ID] = *g.ParentID
			}
		}

		// 从 newParentID 向上爬，检查是否会走到 id
		cursor := newParentID
		visited := map[int64]bool{id: true} // 把自己标记为"已经在路径上"
		for {
			if visited[cursor] {
				return nil, ErrCycleDetected
			}
			parent, hasParent := parentMap[cursor]
			if !hasParent {
				break // 到达顶级，安全
			}
			visited[cursor] = true
			cursor = parent
		}
	}
	// ─────────────────────────────────────────────────────────────

	g, err := s.repo.Update(ctx, userID, id, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("更新 group 失败",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return nil, err
	}

	return s.enrichResult(ctx, g), nil
}

func (s *groupService) Delete(ctx context.Context, userID, id int64) error {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if current.OwnerID != userID {
		return ErrForbidden
	}

	// 级联删除策略：当前采用"拒绝删除"策略
	// 如果分组下有子分组或文档，不允许删除，要求用户先清空。
	childCount, err := s.repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrHasChildren
	}

	snippetCount, err := s.repo.CountSnippets(ctx, id)
	if err != nil {
		return err
	}
	if snippetCount > 0 {
		return ErrHasSnippets
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("删除 group 失败",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// ─── 技术点 1：GetTree — 一次查全部 + O(n) 内存建树 ──────────────
//
//	算法步骤：
//	1. SELECT * FROM groups WHERE owner_id = ?  → 扁平列表
//	2. 遍历一次，为每个 group 建一个 TreeNode，放入 map[id]*TreeNode
//	3. 再遍历一次，把每个 node append 到 parent 的 Children 里
//	4. 没有 parent 的就是 root，收集起来返回
//
//	时间 O(n)，空间 O(n)，用户分组量通常 < 500，毫秒级完成。
func (s *groupService) GetTree(ctx context.Context, userID int64) ([]contract.GroupTreeNode, error) {
	allGroups, err := s.repo.ListAllByOwner(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("获取用户全部 group 失败",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(allGroups) == 0 {
		return []contract.GroupTreeNode{}, nil
	}

	// Step 1: 为每个 group 创建 TreeNode，放入 lookup map
	nodeMap := make(map[int64]*contract.GroupTreeNode, len(allGroups))
	for _, g := range allGroups {
		childCount, _ := s.repo.CountChildren(ctx, g.ID)
		snippetCount, _ := s.repo.CountSnippets(ctx, g.ID)

		nodeMap[g.ID] = &contract.GroupTreeNode{
			ID:            g.ID,
			ParentID:      g.ParentID,
			Name:          g.Name,
			Description:   g.Description,
			SortOrder:     g.SortOrder,
			ChildrenCount: childCount,
			SnippetCount:  snippetCount,
			CreatedAt:     g.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     g.UpdatedAt.Format(time.RFC3339),
			Children:      []contract.GroupTreeNode{}, // 初始化空 slice（前端收到 [] 而非 null）
		}
	}

	// Step 2: 构建父子关系
	var roots []contract.GroupTreeNode
	for _, g := range allGroups {
		node := nodeMap[g.ID]
		if g.ParentID != nil {
			if parent, ok := nodeMap[*g.ParentID]; ok {
				parent.Children = append(parent.Children, *node)
			} else {
				// 孤儿节点（parent 被删但 child 还在），当作顶级处理
				roots = append(roots, *node)
			}
		} else {
			roots = append(roots, *node)
		}
	}

	if roots == nil {
		roots = []contract.GroupTreeNode{}
	}

	return roots, nil
}

// enrichResult 填充子分组和片段计数。
func (s *groupService) enrichResult(ctx context.Context, g *grouprepo.Group) *contract.GroupResult {
	childCount, _ := s.repo.CountChildren(ctx, g.ID)
	snippetCount, _ := s.repo.CountSnippets(ctx, g.ID)

	return &contract.GroupResult{
		ID:            g.ID,
		OwnerID:       g.OwnerID,
		ParentID:      g.ParentID,
		Name:          g.Name,
		Description:   g.Description,
		SortOrder:     g.SortOrder,
		ChildrenCount: childCount,
		SnippetCount:  snippetCount,
		CreatedAt:     g.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     g.UpdatedAt.Format(time.RFC3339),
	}
}
