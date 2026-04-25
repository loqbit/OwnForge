package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	"github.com/loqbit/ownforge/services/notes/internal/platform/storage"
	"github.com/loqbit/ownforge/services/notes/internal/service/upload/contract"
	"go.uber.org/zap"
)

const defaultMaxFileSize = 10 << 20 // 10 MB

var (
	ErrFileTooLarge    = pkgerrs.NewParamErr("file exceeds the 10 MB limit", nil)
	ErrUnsupportedType = pkgerrs.NewParamErr("unsupported file type", nil)
	ErrObjectKeyEmpty  = pkgerrs.NewParamErr("object_key  cannot be empty", nil)
)

// UploadService defines the file upload service interface.
type UploadService interface {
	Upload(ctx context.Context, cmd *contract.UploadCommand, reader io.Reader) (*contract.UploadResult, error)
	Presign(ctx context.Context, cmd *contract.PresignCommand) (*contract.PresignResult, error)
	Complete(ctx context.Context, cmd *contract.CompleteUploadCommand) (*contract.CompleteUploadResult, error)
	// CopyObject copies an object into a new user directory, typically for forks, and returns the new public URL.
	CopyObject(ctx context.Context, srcFileURL string, dstOwnerID int64) (newURL string, err error)
}

// Options contains upload configuration.
type Options struct {
	PresignExpiry time.Duration
	MaxFileSize   int64
	AllowedMIMEs  []string
}

type uploadService struct {
	storage       storage.Storage
	logger        *zap.Logger
	presignExpiry time.Duration
	maxFileSize   int64
	allowedMIMEs  []string
}

// NewUploadService creates an UploadService instance.
func NewUploadService(s storage.Storage, opts Options, logger *zap.Logger) UploadService {
	maxFileSize := opts.MaxFileSize
	if maxFileSize <= 0 {
		maxFileSize = defaultMaxFileSize
	}

	presignExpiry := opts.PresignExpiry
	if presignExpiry <= 0 {
		presignExpiry = 10 * time.Minute
	}

	allowed := opts.AllowedMIMEs
	if len(allowed) == 0 {
		allowed = []string{"image/", "application/pdf", "text/"}
	}

	return &uploadService{
		storage:       s,
		logger:        logger,
		presignExpiry: presignExpiry,
		maxFileSize:   maxFileSize,
		allowedMIMEs:  allowed,
	}
}

// Upload runs the core file upload and validation flow.
// It first enforces business-level safety checks on file type and size.
// After validation, it generates a collision-resistant object key.
// It then streams the data into storage and builds the success response expected by the frontend.
func (s *uploadService) Upload(ctx context.Context, cmd *contract.UploadCommand, reader io.Reader) (*contract.UploadResult, error) {
	if err := s.validateMIME(cmd.ContentType); err != nil {
		return nil, err
	}
	if cmd.Size > s.maxFileSize {
		return nil, ErrFileTooLarge
	}

	key := buildKey(cmd.OwnerID, cmd.Filename)
	url, err := s.storage.Upload(ctx, key, reader, cmd.Size, cmd.ContentType)
	if err != nil {
		s.logger.Error("failed to upload file", zap.String("key", key), zap.Error(err))
		return nil, pkgerrs.NewServerErr(fmt.Errorf("failed to upload file: %w", err))
	}

	result := &contract.UploadResult{
		URL:      url,
		Filename: cmd.Filename,
		Size:     cmd.Size,
		MimeType: cmd.ContentType,
	}
	return result, nil
}

// Presign generates a presigned URL for direct MinIO uploads.
func (s *uploadService) Presign(ctx context.Context, cmd *contract.PresignCommand) (*contract.PresignResult, error) {
	if err := s.validateMIME(cmd.ContentType); err != nil {
		return nil, err
	}
	if cmd.Size > s.maxFileSize {
		return nil, ErrFileTooLarge
	}

	key := buildKey(cmd.OwnerID, cmd.Filename)
	url, headers, err := s.storage.PresignedPutObject(ctx, key, s.presignExpiry, cmd.ContentType, cmd.Size)
	if err != nil {
		s.logger.Error("failed to generate presigned upload URL", zap.String("key", key), zap.Error(err))
		return nil, pkgerrs.NewServerErr(fmt.Errorf("failed to generate presigned upload URL: %w", err))
	}

	return &contract.PresignResult{
		URL:       url,
		ObjectKey: key,
		ExpiresAt: time.Now().Add(s.presignExpiry).UTC().Format(time.RFC3339),
		PublicURL: s.storage.PublicURL(key),
		Headers:   headers,
	}, nil
}

// Complete handles the direct-upload callback and currently only validates metadata before returning a public URL.
func (s *uploadService) Complete(_ context.Context, cmd *contract.CompleteUploadCommand) (*contract.CompleteUploadResult, error) {
	if strings.TrimSpace(cmd.ObjectKey) == "" {
		return nil, ErrObjectKeyEmpty
	}
	if err := s.validateMIME(cmd.ContentType); err != nil {
		return nil, err
	}
	if cmd.Size > s.maxFileSize {
		return nil, ErrFileTooLarge
	}

	return &contract.CompleteUploadResult{
		URL:       s.storage.PublicURL(cmd.ObjectKey),
		ObjectKey: cmd.ObjectKey,
		Filename:  cmd.Filename,
		Size:      cmd.Size,
		MimeType:  cmd.ContentType,
	}, nil
}

// CopyObject copies an object into a new user directory and returns the new public URL.
// srcFileURL is a public URL such as http://host/bucket/u/123/2026/04/xxx.png,
// from which the object key starting with u/ is extracted before calling storage.Copy with a new key.
func (s *uploadService) CopyObject(ctx context.Context, srcFileURL string, dstOwnerID int64) (string, error) {
	srcKey := extractObjectKey(srcFileURL)
	if srcKey == "" {
		return "", pkgerrs.NewParamErr("unable to parse source file path", nil)
	}

	ext := filepath.Ext(srcKey)
	baseName := filepath.Base(srcKey)
	safeName := safeFilename(strings.TrimSuffix(baseName, ext))
	if safeName == "" {
		safeName = "copy"
	}
	now := time.Now()
	dstKey := fmt.Sprintf("u/%d/%d/%02d/%s-%s%s", dstOwnerID, now.Year(), now.Month(), uuid.NewString(), safeName, ext)

	if err := s.storage.Copy(ctx, srcKey, dstKey); err != nil {
		s.logger.Error("failed to copy object", zap.String("src", srcKey), zap.String("dst", dstKey), zap.Error(err))
		return "", pkgerrs.NewServerErr(fmt.Errorf("failed to copy file: %w", err))
	}

	return s.storage.PublicURL(dstKey), nil
}

// extractObjectKey extracts the object key starting with u/ from a public URL.
func extractObjectKey(fileURL string) string {
	idx := strings.Index(fileURL, "u/")
	if idx < 0 {
		return ""
	}
	return fileURL[idx:]
}

// validateMIME validates whether the file MIME type is allowed.
// This mainly prevents users from disguising malicious executable content as normal uploads by allowing only whitelisted MIME prefixes.
func (s *uploadService) validateMIME(mime string) error {
	for _, allowed := range s.allowedMIMEs {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if strings.HasSuffix(allowed, "/*") && strings.HasPrefix(mime, strings.TrimSuffix(allowed, "*")) {
			return nil
		}
		if strings.HasSuffix(allowed, "/") && strings.HasPrefix(mime, allowed) {
			return nil
		}
		if mime == allowed {
			return nil
		}
	}
	return ErrUnsupportedType
}

// buildKey generates a unique object key for object storage.
// Design notes:
// - Use the user ID as the first-level directory for logical isolation.
// - Split the next directory levels by year and month to avoid overloading a single directory.
// - Use the current nanosecond timestamp in the filename to avoid collisions during concurrent uploads.
func buildKey(ownerID int64, filename string) string {
	ext := filepath.Ext(filename)
	now := time.Now()
	name := safeFilename(strings.TrimSuffix(filename, ext))
	if name == "" {
		name = "upload"
	}
	// Format: u/{owner_id}/{year}/{month}/{uuid}-{safeName}{ext}
	return fmt.Sprintf("u/%d/%d/%02d/%s-%s%s", ownerID, now.Year(), now.Month(), uuid.NewString(), name, ext)
}

var invalidFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFilename(name string) string {
	name = invalidFilenameChars.ReplaceAllString(strings.TrimSpace(name), "-")
	name = strings.Trim(name, "-.")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

// Enforce the interface check at compile time.
var _ UploadService = (*uploadService)(nil)

// Avoid an unused import for the errors package.
var _ = errors.New
