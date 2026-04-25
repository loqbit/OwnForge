package group

import (
	"context"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	"github.com/loqbit/ownforge/services/notes/internal/platform/idgen"
	grouprepo "github.com/loqbit/ownforge/services/notes/internal/repository/group"
	"github.com/loqbit/ownforge/services/notes/internal/service/group/contract"

	"go.uber.org/zap"
)

// Domain errors.
var (
	ErrIDGeneration  = pkgerrs.NewServerErr(errors.New("failed to generate group ID"))
	ErrForbidden     = pkgerrs.New(pkgerrs.Forbidden, "no permission to operate on this group", nil)
	ErrHasChildren   = pkgerrs.NewParamErr("the group has child groups; delete or move them first", nil)
	ErrHasSnippets   = pkgerrs.NewParamErr("the group contains documents; delete or move them first", nil)
	ErrNameRequired  = pkgerrs.NewParamErr("group name cannot be empty", nil)
	ErrCycleDetected = pkgerrs.NewParamErr("cycle detected: a group cannot be moved under its own descendant", nil)
)

// GroupService defines the group service interface.
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

// NewGroupService creates a GroupService instance.
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
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate group ID", zap.Error(err))
		return nil, ErrIDGeneration
	}

	params := &contract.CreateGroupCommand{
		ParentID:    req.ParentID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}

	g, err := s.repo.Create(ctx, id, userID, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to create group",
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
		commonlogger.Ctx(ctx, s.logger).Error("failed to query user groups",
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

	// Note 2: cycle prevention
	// Scenario: moving A under its descendant C would create a cycle: A -> C -> ... -> A.
	// Algorithm: start from the target parent and walk upward through the parent_id chain,
	// and if the walk reaches the current id, a cycle would be formed, so reject it.
	// Reaching nil means the path ends at the root and is safe.
	// Complexity: O(depth), worst-case O(n), though real directory depth rarely exceeds 10 levels.
	if params.ParentID != nil {
		newParentID := *params.ParentID

		// A group cannot be moved under itself.
		if newParentID == id {
			return nil, ErrCycleDetected
		}

		// The ancestor chain must be checked by querying everything once, building a parent map, and then walking upward.
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

		// Walk upward from newParentID and check whether the path reaches id.
		cursor := newParentID
		visited := map[int64]bool{id: true} // Mark the current node as already on the path.
		for {
			if visited[cursor] {
				return nil, ErrCycleDetected
			}
			parent, hasParent := parentMap[cursor]
			if !hasParent {
				break // Reached the root; the structure is safe.
			}
			visited[cursor] = true
			cursor = parent
		}
	}
	// ─────────────────────────────────────────────────────────────

	g, err := s.repo.Update(ctx, userID, id, params)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to update group",
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

	// Cascading delete policy: currently this uses a reject-delete strategy.
	// Deletion is rejected when the group still contains child groups or documents; the user must clear it first.
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
		commonlogger.Ctx(ctx, s.logger).Error("failed to delete group",
			zap.Int64("id", id),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// Note 1: GetTree — query everything once and build the tree in O(n) memory
//
// Algorithm:
// 1. SELECT * FROM groups WHERE owner_id = ?  -> flat list
// 2. Iterate once, create a TreeNode for each group, and store it in map[id]*TreeNode
// 3. Iterate again and append each node to its parent's Children
// 4. Nodes without a parent are roots and are collected for the return value
//
// Time O(n), space O(n). User group counts are usually under 500, so this completes in milliseconds.
func (s *groupService) GetTree(ctx context.Context, userID int64) ([]contract.GroupTreeNode, error) {
	allGroups, err := s.repo.ListAllByOwner(ctx, userID)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to fetch all user groups",
			zap.Int64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(allGroups) == 0 {
		return []contract.GroupTreeNode{}, nil
	}

	// Step 1: create a TreeNode for each group and store it in the lookup map.
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
			Children:      []contract.GroupTreeNode{}, // Initialize an empty slice so the frontend receives [] instead of null.
		}
	}

	// Step 2: build parent-child relationships.
	var roots []contract.GroupTreeNode
	for _, g := range allGroups {
		node := nodeMap[g.ID]
		if g.ParentID != nil {
			if parent, ok := nodeMap[*g.ParentID]; ok {
				parent.Children = append(parent.Children, *node)
			} else {
				// Orphaned nodes, where the parent was removed but the child remains, are treated as top-level nodes.
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

// enrichResult fills in child-group and snippet counts.
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
