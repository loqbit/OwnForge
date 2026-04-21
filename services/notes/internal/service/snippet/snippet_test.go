package snippet

import (
	"context"
	"testing"
	"time"

	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	snippetrepo "github.com/ownforge/ownforge/services/notes/internal/repository/snippet"
	tagrepo "github.com/ownforge/ownforge/services/notes/internal/repository/tag"
	"github.com/ownforge/ownforge/services/notes/internal/service/snippet/contract"
	tagcontract "github.com/ownforge/ownforge/services/notes/internal/service/tag/contract"
	"go.uber.org/zap"
)

func TestGetMineByIDRejectsForeignOwner(t *testing.T) {
	svc := NewSnippetService(&fakeSnippetRepo{
		snippet: &snippetrepo.Snippet{ID: 1, OwnerID: 200},
	}, &fakeTagRepo{}, nil, nil, zap.NewNop())

	_, err := svc.GetMineByID(context.Background(), 100, 1)
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}




func TestSetFavoriteRejectsTrashedSnippet(t *testing.T) {
	deletedAt := time.Now()
	repo := &fakeSnippetRepo{
		snippet: &snippetrepo.Snippet{
			ID:        1,
			OwnerID:   100,
			DeletedAt: &deletedAt,
		},
	}
	svc := NewSnippetService(repo, &fakeTagRepo{}, nil, nil, zap.NewNop())

	err := svc.SetFavorite(context.Background(), 100, 1, true)
	if err != ErrTrashed {
		t.Fatalf("expected ErrTrashed, got %v", err)
	}
	if repo.setFavoriteCalled {
		t.Fatalf("expected SetFavorite repository method not to be called")
	}
}

func TestRestoreRejectsActiveSnippet(t *testing.T) {
	repo := &fakeSnippetRepo{
		snippet: &snippetrepo.Snippet{
			ID:      1,
			OwnerID: 100,
		},
	}
	svc := NewSnippetService(repo, &fakeTagRepo{}, nil, nil, zap.NewNop())

	err := svc.Restore(context.Background(), 100, 1)
	if err != ErrNotTrashed {
		t.Fatalf("expected ErrNotTrashed, got %v", err)
	}
	if repo.restoreCalled {
		t.Fatalf("expected Restore repository method not to be called")
	}
}

type fakeSnippetRepo struct {
	snippet           *snippetrepo.Snippet
	err               error
	setFavoriteCalled bool
	restoreCalled     bool
}

func (f *fakeSnippetRepo) Create(context.Context, int64, int64, *contract.CreateSnippetCommand) (*snippetrepo.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) GetByID(context.Context, int64) (*snippetrepo.Snippet, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.snippet == nil {
		return nil, sharedrepo.ErrNoRows
	}
	return f.snippet, nil
}

func (f *fakeSnippetRepo) ListByOwner(context.Context, int64) ([]snippetrepo.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) ListFiltered(context.Context, int64, *contract.ListQuery) ([]snippetrepo.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) Update(context.Context, int64, int64, *contract.UpdateSnippetCommand) (*snippetrepo.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) Delete(context.Context, int64, int64) error {
	return nil
}

func (f *fakeSnippetRepo) SoftDelete(context.Context, int64, int64) error {
	return nil
}

func (f *fakeSnippetRepo) Restore(context.Context, int64, int64) error {
	f.restoreCalled = true
	return nil
}

func (f *fakeSnippetRepo) SetFavorite(context.Context, int64, int64, bool) error {
	f.setFavoriteCalled = true
	return nil
}

func (f *fakeSnippetRepo) Move(context.Context, int64, int64, *int64, int) (*snippetrepo.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepo) MaxSortOrderInGroup(context.Context, int64, *int64) (int, error) {
	return 0, nil
}

func (f *fakeSnippetRepo) SetTags(context.Context, int64, []int64) error {
	return nil
}

func (f *fakeSnippetRepo) GetTagIDs(context.Context, int64) ([]int64, error) {
	return []int64{}, nil
}

type fakeTagRepo struct{}

func (f *fakeTagRepo) Create(context.Context, int64, int64, *tagcontract.CreateTagCommand) (*tagrepo.Tag, error) {
	return nil, nil
}

func (f *fakeTagRepo) GetByID(context.Context, int64) (*tagrepo.Tag, error) {
	return nil, nil
}

func (f *fakeTagRepo) ListByOwner(context.Context, int64) ([]tagrepo.Tag, error) {
	return nil, nil
}

func (f *fakeTagRepo) ListByIDs(context.Context, int64, []int64) ([]tagrepo.Tag, error) {
	return nil, nil
}

func (f *fakeTagRepo) Update(context.Context, int64, int64, *tagcontract.UpdateTagCommand) (*tagrepo.Tag, error) {
	return nil, nil
}

func (f *fakeTagRepo) Delete(context.Context, int64, int64) error {
	return nil
}
