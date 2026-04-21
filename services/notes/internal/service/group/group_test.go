package group

import (
	"context"
	"testing"

	grouprepo "github.com/ownforge/ownforge/services/notes/internal/repository/group"
	"github.com/ownforge/ownforge/services/notes/internal/service/group/contract"
	"go.uber.org/zap"
)

func TestUpdateRejectsCycleMove(t *testing.T) {
	parentID := int64(1)
	repo := &fakeGroupRepo{
		group: &grouprepo.Group{
			ID:      1,
			OwnerID: 100,
			Name:    "Root",
		},
		allGroups: []grouprepo.Group{
			{ID: 1, OwnerID: 100, Name: "Root"},
			{ID: 2, OwnerID: 100, Name: "Child", ParentID: &parentID},
		},
	}
	svc := NewGroupService(repo, nil, zap.NewNop())

	_, err := svc.Update(context.Background(), 100, 1, &contract.UpdateGroupCommand{
		Name:     "Root",
		ParentID: int64Ptr(2),
	})
	if err != ErrCycleDetected {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
	if repo.updateCalled {
		t.Fatalf("expected repository Update not to be called")
	}
}

func TestDeleteRejectsGroupWithChildren(t *testing.T) {
	repo := &fakeGroupRepo{
		group: &grouprepo.Group{
			ID:      1,
			OwnerID: 100,
			Name:    "Root",
		},
		childCount: 1,
	}
	svc := NewGroupService(repo, nil, zap.NewNop())

	err := svc.Delete(context.Background(), 100, 1)
	if err != ErrHasChildren {
		t.Fatalf("expected ErrHasChildren, got %v", err)
	}
	if repo.deleteCalled {
		t.Fatalf("expected repository Delete not to be called")
	}
}

func TestDeleteRejectsGroupWithSnippets(t *testing.T) {
	repo := &fakeGroupRepo{
		group: &grouprepo.Group{
			ID:      1,
			OwnerID: 100,
			Name:    "Root",
		},
		snippetCount: 2,
	}
	svc := NewGroupService(repo, nil, zap.NewNop())

	err := svc.Delete(context.Background(), 100, 1)
	if err != ErrHasSnippets {
		t.Fatalf("expected ErrHasSnippets, got %v", err)
	}
	if repo.deleteCalled {
		t.Fatalf("expected repository Delete not to be called")
	}
}

type fakeGroupRepo struct {
	group        *grouprepo.Group
	allGroups    []grouprepo.Group
	childCount   int
	snippetCount int
	updateCalled bool
	deleteCalled bool
}

func (f *fakeGroupRepo) Create(context.Context, int64, int64, *contract.CreateGroupCommand) (*grouprepo.Group, error) {
	return nil, nil
}

func (f *fakeGroupRepo) GetByID(context.Context, int64) (*grouprepo.Group, error) {
	return f.group, nil
}

func (f *fakeGroupRepo) ListByOwner(context.Context, int64, *int64) ([]grouprepo.Group, error) {
	return nil, nil
}

func (f *fakeGroupRepo) ListAllByOwner(context.Context, int64) ([]grouprepo.Group, error) {
	return f.allGroups, nil
}

func (f *fakeGroupRepo) Update(context.Context, int64, int64, *contract.UpdateGroupCommand) (*grouprepo.Group, error) {
	f.updateCalled = true
	return f.group, nil
}

func (f *fakeGroupRepo) Delete(context.Context, int64, int64) error {
	f.deleteCalled = true
	return nil
}

func (f *fakeGroupRepo) CountChildren(context.Context, int64) (int, error) {
	return f.childCount, nil
}

func (f *fakeGroupRepo) CountSnippets(context.Context, int64) (int, error) {
	return f.snippetCount, nil
}

func int64Ptr(v int64) *int64 {
	return &v
}
