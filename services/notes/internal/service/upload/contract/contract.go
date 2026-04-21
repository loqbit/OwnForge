package contract

// UploadCommand 上传请求。
type UploadCommand struct {
	OwnerID     int64
	Filename    string
	Size        int64
	ContentType string
}

// PresignCommand 上传预签名请求。
type PresignCommand struct {
	OwnerID     int64
	Filename    string
	Size        int64
	ContentType string
}

// CompleteUploadCommand 直传完成回调请求。
type CompleteUploadCommand struct {
	OwnerID     int64
	ObjectKey   string
	Filename    string
	Size        int64
	ContentType string
	SnippetID   *int64
}

// UploadResult 上传结果。
type UploadResult struct {
	URL          string
	Filename     string
	Size         int64
	MimeType     string
	ThumbnailURL string // 仅图片类型有值
}

// PresignResult 上传预签名结果。
type PresignResult struct {
	URL       string
	ObjectKey string
	ExpiresAt string
	PublicURL string
	Headers   map[string]string
}

// CompleteUploadResult 直传完成结果。
type CompleteUploadResult struct {
	URL          string
	ObjectKey    string
	Filename     string
	Size         int64
	MimeType     string
	ThumbnailURL string
}
