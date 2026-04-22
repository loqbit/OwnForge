package contract

// UploadCommand is the upload request payload.
type UploadCommand struct {
	OwnerID     int64
	Filename    string
	Size        int64
	ContentType string
}

// PresignCommand is the presign request payload.
type PresignCommand struct {
	OwnerID     int64
	Filename    string
	Size        int64
	ContentType string
}

// CompleteUploadCommand is the callback payload for completed direct uploads.
type CompleteUploadCommand struct {
	OwnerID     int64
	ObjectKey   string
	Filename    string
	Size        int64
	ContentType string
	SnippetID   *int64
}

// UploadResult is returned after an upload.
type UploadResult struct {
	URL          string
	Filename     string
	Size         int64
	MimeType     string
	ThumbnailURL string // Set only for image uploads.
}

// PresignResult is returned after presigning an upload.
type PresignResult struct {
	URL       string
	ObjectKey string
	ExpiresAt string
	PublicURL string
	Headers   map[string]string
}

// CompleteUploadResult is returned after direct upload completion.
type CompleteUploadResult struct {
	URL          string
	ObjectKey    string
	Filename     string
	Size         int64
	MimeType     string
	ThumbnailURL string
}
