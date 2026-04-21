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
	pkgerrs "github.com/luckysxx/common/errs"
	"github.com/ownforge/ownforge/services/notes/internal/platform/storage"
	"github.com/ownforge/ownforge/services/notes/internal/service/upload/contract"
	"go.uber.org/zap"
)

const defaultMaxFileSize = 10 << 20 // 10 MB

var (
	ErrFileTooLarge    = pkgerrs.NewParamErr("文件超过 10MB 限制", nil)
	ErrUnsupportedType = pkgerrs.NewParamErr("不支持的文件类型", nil)
	ErrObjectKeyEmpty  = pkgerrs.NewParamErr("object_key 不能为空", nil)
)

// UploadService 文件上传业务接口。
type UploadService interface {
	Upload(ctx context.Context, cmd *contract.UploadCommand, reader io.Reader) (*contract.UploadResult, error)
	Presign(ctx context.Context, cmd *contract.PresignCommand) (*contract.PresignResult, error)
	Complete(ctx context.Context, cmd *contract.CompleteUploadCommand) (*contract.CompleteUploadResult, error)
	// CopyObject 复制对象到新用户目录下（用于 fork），返回新文件的公开 URL。
	CopyObject(ctx context.Context, srcFileURL string, dstOwnerID int64) (newURL string, err error)
}

// Options 上传配置。
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

// NewUploadService 创建 UploadService 实例。
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

// Upload 执行核心的文件上传与校验流程。
// 首先会对文件格式和大小进行业务级安全拦截；
// 校验通过后动态生成防冲突文件路径（Object Key）；
// 最终将数据流推送到下层存储服务进行最终落盘，并构建前端需要的成功结果。
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
		s.logger.Error("上传文件失败", zap.String("key", key), zap.Error(err))
		return nil, pkgerrs.NewServerErr(fmt.Errorf("上传文件失败: %w", err))
	}

	result := &contract.UploadResult{
		URL:      url,
		Filename: cmd.Filename,
		Size:     cmd.Size,
		MimeType: cmd.ContentType,
	}
	return result, nil
}

// Presign 生成直传 MinIO 的预签名 URL。
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
		s.logger.Error("生成上传预签名失败", zap.String("key", key), zap.Error(err))
		return nil, pkgerrs.NewServerErr(fmt.Errorf("生成上传预签名失败: %w", err))
	}

	return &contract.PresignResult{
		URL:       url,
		ObjectKey: key,
		ExpiresAt: time.Now().Add(s.presignExpiry).UTC().Format(time.RFC3339),
		PublicURL: s.storage.PublicURL(key),
		Headers:   headers,
	}, nil
}

// Complete 完成直传回调，当前阶段仅做元数据校验并返回公开地址。
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

// CopyObject 复制对象到新用户目录下，返回新公开 URL。
// srcFileURL 是形如 http://host/bucket/u/123/2026/04/xxx.png 的公开地址，
// 从中提取 object key（u/ 开头的部分），生成新 key 后调用 storage.Copy。
func (s *uploadService) CopyObject(ctx context.Context, srcFileURL string, dstOwnerID int64) (string, error) {
	srcKey := extractObjectKey(srcFileURL)
	if srcKey == "" {
		return "", pkgerrs.NewParamErr("无法解析源文件路径", nil)
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
		s.logger.Error("复制对象失败", zap.String("src", srcKey), zap.String("dst", dstKey), zap.Error(err))
		return "", pkgerrs.NewServerErr(fmt.Errorf("复制文件失败: %w", err))
	}

	return s.storage.PublicURL(dstKey), nil
}

// extractObjectKey 从公开 URL 中提取 object key（u/ 开头的部分）。
func extractObjectKey(fileURL string) string {
	idx := strings.Index(fileURL, "u/")
	if idx < 0 {
		return ""
	}
	return fileURL[idx:]
}

// validateMIME 校验文件的 MIME 类型是否合法。
// 主要为了防止用户试图伪装成正常文件上传可执行脚本等恶意内容，只允许白名单中的前缀通行。
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

// buildKey 生成存储在对象存储系统中的唯一文件路径（Object Key）。
// 设计考量:
// - 使用用户 ID 作为第一级目录实现逻辑隔离；
// - 按年月划分第二、三级目录防止单目录下文件过多影响性能；
// - 使用当前纳秒级时间戳作为文件名防止并发上传产生的文件名冲突覆盖。
func buildKey(ownerID int64, filename string) string {
	ext := filepath.Ext(filename)
	now := time.Now()
	name := safeFilename(strings.TrimSuffix(filename, ext))
	if name == "" {
		name = "upload"
	}
	// 格式：u/{owner_id}/{year}/{month}/{uuid}-{safeName}{ext}
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

// 确保编译期接口检查。
var _ UploadService = (*uploadService)(nil)

// 避免 errors 包未使用。
var _ = errors.New
