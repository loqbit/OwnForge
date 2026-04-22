package dto

// GetProfileResponse 表示获取个人资料接口的响应体。
type GetProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Birthday  string `json:"birthday"`
	UpdatedAt string `json:"updated_at"`
}

// UpdateProfileRequest 表示更新个人资料接口的请求体。
type UpdateProfileRequest struct {
	Nickname  string `json:"nickname" binding:"omitempty,max=32"`
	AvatarURL string `json:"avatar_url" binding:"omitempty,max=512"`
	Bio       string `json:"bio" binding:"omitempty,max=256"`
	Birthday  string `json:"birthday" binding:"omitempty,len=10"`
}

// UpdateProfileResponse 表示更新个人资料后的响应体。
type UpdateProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Birthday  string `json:"birthday"`
	UpdatedAt string `json:"updated_at"`
}
