package dto

// GetProfileResponse represents the response body for the get-profile API.
type GetProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Birthday  string `json:"birthday"`
	UpdatedAt string `json:"updated_at"`
}

// UpdateProfileRequest represents the request body for the update-profile API.
type UpdateProfileRequest struct {
	Nickname  string `json:"nickname" binding:"omitempty,max=32"`
	AvatarURL string `json:"avatar_url" binding:"omitempty,max=512"`
	Bio       string `json:"bio" binding:"omitempty,max=256"`
	Birthday  string `json:"birthday" binding:"omitempty,len=10"`
}

// UpdateProfileResponse represents the response body after updating the profile.
type UpdateProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Birthday  string `json:"birthday"`
	UpdatedAt string `json:"updated_at"`
}
