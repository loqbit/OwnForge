package dto

// RegisterRequest 表示用户注册的 HTTP 请求体。
type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=20,alphanum"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest 表示用户登录的 HTTP 请求体。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// LogoutRequest 表示用户登出的 HTTP 请求体。
type LogoutRequest struct {
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// RegisterResponse 表示用户注册的 HTTP 响应体。
type RegisterResponse struct {
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

// LoginResponse 表示用户登录的 HTTP 响应体。
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
}

type RefreshTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ChangePasswordRequest 表示修改密码的 HTTP 请求体。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePasswordResponse 表示修改密码的 HTTP 响应体。
type ChangePasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// LogoutAllSessionsResponse 表示退出全部设备的 HTTP 响应体。
type LogoutAllSessionsResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// BindEmailRequest 表示绑定邮箱的 HTTP 请求体。
type BindEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// BindEmailResponse 表示绑定邮箱的 HTTP 响应体。
type BindEmailResponse struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// SetPasswordRequest 表示设置密码的 HTTP 请求体。
type SetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// SetPasswordResponse 表示设置密码的 HTTP 响应体。
type SetPasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}
