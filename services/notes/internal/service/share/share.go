package share

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	commonlogger "github.com/loqbit/ownforge/pkg/logger"
	"github.com/loqbit/ownforge/services/notes/internal/platform/idgen"
	sharerepo "github.com/loqbit/ownforge/services/notes/internal/repository/share"
	sharedrepo "github.com/loqbit/ownforge/services/notes/internal/repository/shared"
	snippetrepo "github.com/loqbit/ownforge/services/notes/internal/repository/snippet"
	"github.com/loqbit/ownforge/services/notes/internal/service/share/contract"
	snippetcontract "github.com/loqbit/ownforge/services/notes/internal/service/snippet/contract"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrIDGeneration       = pkgerrs.NewServerErr(errors.New("failed to generate share ID"))
	ErrSnippetForbidden   = pkgerrs.New(pkgerrs.Forbidden, "no permission to share this document", nil)
	ErrShareForbidden     = pkgerrs.New(pkgerrs.Forbidden, "no permission to operate on this share", nil)
	ErrInvalidKind        = pkgerrs.NewParamErr("kind only supports article or template", nil)
	ErrPasswordRequired   = pkgerrs.New(pkgerrs.Unauthorized, "this share requires a password", nil)
	ErrInvalidPassword    = pkgerrs.New(pkgerrs.Unauthorized, "incorrect share password", nil)
	ErrShareExpired       = pkgerrs.New(pkgerrs.Gone, "share has expired", nil)
	ErrSnippetIDRequired  = pkgerrs.NewParamErr("snippet_id cannot be empty", nil)
	ErrPasswordTooLong    = pkgerrs.NewParamErr("share password cannot exceed 72 characters", nil)
	ErrInvalidExpiresAt   = pkgerrs.NewParamErr("invalid expires_at format, RFC3339 time required", nil)
	ErrTokenGeneration    = pkgerrs.NewServerErr(errors.New("failed to generate share token"))
	ErrShareAlreadyExists = pkgerrs.New(pkgerrs.ParamErr, "an active share already exists for this document", nil)
)

// Service defines the share service interface.
type Service interface {
	Create(ctx context.Context, userID int64, cmd *contract.CreateShareCommand) (*contract.ShareResult, error)
	ListMine(ctx context.Context, userID int64, query *contract.ListSharesQuery) ([]contract.ShareResult, error)
	Delete(ctx context.Context, userID, id int64) error
	GetPublicByToken(ctx context.Context, token, password string) (*contract.PublicShareResult, error)
	GetSourceByToken(ctx context.Context, token, password string) (*contract.ShareSource, error)
	IncrementForkCount(ctx context.Context, shareID int64) error
}

type shareService struct {
	repo        sharerepo.Repository
	snippetRepo snippetrepo.Repository
	idgen       idgen.Client
	logger      *zap.Logger
}

// NewService creates a share service instance.
func NewService(repo sharerepo.Repository, snippetRepo snippetrepo.Repository, idgenClient idgen.Client, logger *zap.Logger) Service {
	return &shareService{
		repo:        repo,
		snippetRepo: snippetRepo,
		idgen:       idgenClient,
		logger:      logger,
	}
}

func (s *shareService) Create(ctx context.Context, userID int64, cmd *contract.CreateShareCommand) (*contract.ShareResult, error) {
	if cmd.SnippetID <= 0 {
		return nil, ErrSnippetIDRequired
	}

	kind := normalizeKind(cmd.Kind)
	if kind == "" {
		return nil, ErrInvalidKind
	}

	snippet, err := s.snippetRepo.GetByID(ctx, cmd.SnippetID)
	if err != nil {
		return nil, err
	}
	if snippet.OwnerID != userID {
		return nil, ErrSnippetForbidden
	}

	now := time.Now()
	if existing, err := s.repo.FindActiveToken(ctx, userID, cmd.SnippetID, kind, now); err == nil && existing != nil {
		return toResult(existing), nil
	} else if err != nil && !sharedrepo.IsNotFoundError(err) {
		return nil, err
	}

	id, err := s.idgen.NextID(ctx)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate share ID", zap.Error(err))
		return nil, ErrIDGeneration
	}

	token, err := generateToken()
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to generate share token", zap.Error(err))
		return nil, ErrTokenGeneration
	}

	share := &sharerepo.Share{
		ID:        id,
		Token:     token,
		Kind:      kind,
		SnippetID: cmd.SnippetID,
		OwnerID:   userID,
	}

	if expiresAt := strings.TrimSpace(cmd.ExpiresAt); expiresAt != "" {
		t, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, ErrInvalidExpiresAt
		}
		share.ExpiresAt = &t
	}

	if password := strings.TrimSpace(cmd.Password); password != "" {
		if len(password) > 72 {
			return nil, ErrPasswordTooLong
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			commonlogger.Ctx(ctx, s.logger).Error("failed to hash share password", zap.Error(err))
			return nil, pkgerrs.NewServerErr(err)
		}
		share.PasswordHash = string(hash)
	}

	created, err := s.repo.Create(ctx, share)
	if err != nil {
		commonlogger.Ctx(ctx, s.logger).Error("failed to create share", zap.Int64("snippet_id", cmd.SnippetID), zap.Error(err))
		return nil, err
	}

	return toResult(created), nil
}

func (s *shareService) ListMine(ctx context.Context, userID int64, query *contract.ListSharesQuery) ([]contract.ShareResult, error) {
	kind := ""
	if query != nil {
		kind = normalizeKind(query.Kind)
		if query.Kind != "" && kind == "" {
			return nil, ErrInvalidKind
		}
	}

	list, err := s.repo.ListByOwner(ctx, userID, kind)
	if err != nil {
		return nil, err
	}

	results := make([]contract.ShareResult, 0, len(list))
	for i := range list {
		results = append(results, *toResult(&list[i]))
	}

	return results, nil
}

func (s *shareService) Delete(ctx context.Context, userID, id int64) error {
	share, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if share.OwnerID != userID {
		return ErrShareForbidden
	}
	return s.repo.Delete(ctx, userID, id)
}

func (s *shareService) GetPublicByToken(ctx context.Context, token, password string) (*contract.PublicShareResult, error) {
	source, err := s.getSourceByToken(ctx, token, password)
	if err != nil {
		return nil, err
	}

	if err := s.repo.IncrementViewCount(ctx, source.Share.ID); err != nil {
		commonlogger.Ctx(ctx, s.logger).Warn("failed to increment share view count", zap.Int64("share_id", source.Share.ID), zap.Error(err))
	} else {
		source.Share.ViewCount++
	}

	return &contract.PublicShareResult{
		Share:   source.Share,
		Snippet: source.Snippet,
	}, nil
}

func (s *shareService) GetSourceByToken(ctx context.Context, token, password string) (*contract.ShareSource, error) {
	return s.getSourceByToken(ctx, token, password)
}

func (s *shareService) IncrementForkCount(ctx context.Context, shareID int64) error {
	return s.repo.IncrementForkCount(ctx, shareID)
}

func (s *shareService) getSourceByToken(ctx context.Context, token, password string) (*contract.ShareSource, error) {
	share, err := s.repo.GetByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, ErrShareExpired
	}

	if share.PasswordHash != "" {
		if strings.TrimSpace(password) == "" {
			return nil, ErrPasswordRequired
		}
		if err := bcrypt.CompareHashAndPassword([]byte(share.PasswordHash), []byte(password)); err != nil {
			return nil, ErrInvalidPassword
		}
	}

	snippet, err := s.snippetRepo.GetByID(ctx, share.SnippetID)
	if err != nil {
		return nil, err
	}

	return &contract.ShareSource{
		Share:   toResult(share),
		Snippet: toSnippetResult(snippet),
	}, nil
}

func normalizeKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "", "article":
		return "article"
	case "template":
		return "template"
	default:
		return ""
	}
}

func toResult(share *sharerepo.Share) *contract.ShareResult {
	if share == nil {
		return nil
	}

	result := &contract.ShareResult{
		ID:          share.ID,
		Token:       share.Token,
		Kind:        share.Kind,
		SnippetID:   share.SnippetID,
		OwnerID:     share.OwnerID,
		HasPassword: share.PasswordHash != "",
		ViewCount:   share.ViewCount,
		ForkCount:   share.ForkCount,
		CreatedAt:   share.CreatedAt.Format(time.RFC3339),
	}
	if share.ExpiresAt != nil {
		result.ExpiresAt = share.ExpiresAt.Format(time.RFC3339)
	}
	return result
}

func toSnippetResult(s *snippetrepo.Snippet) *snippetcontract.SnippetResult {
	if s == nil {
		return nil
	}
	return &snippetcontract.SnippetResult{
		ID:        s.ID,
		OwnerID:   s.OwnerID,
		Type:      s.Type,
		Title:     s.Title,
		Content:   s.Content,
		FileURL:   s.FileURL,
		FileSize:  s.FileSize,
		MimeType:  s.MimeType,
		Language:  s.Language,
		GroupID:   s.GroupID,
		SortOrder: s.SortOrder,
		TagIDs:    s.TagIDs,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

func generateToken() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	token = strings.ReplaceAll(token, "-", "a")
	token = strings.ReplaceAll(token, "_", "b")
	return token, nil
}
