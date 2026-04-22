package dto

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceId string `json:"device_id" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type ExchangeSSORequest struct {
	AppCode  string `json:"app_code" binding:"required"`
	DeviceId string `json:"device_id" binding:"required"`
}

type ExchangeSSOResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
}

type LogoutRequest struct {
	AppCode  string `json:"app_code"`
	DeviceId string `json:"device_id"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type ChangePasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

type LogoutAllSessionsResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

type BindEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type BindEmailResponse struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

type SetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type SetPasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// ---- 手机认证相关 DTO ----

type SendPhoneCodeRequest struct {
	Phone string `json:"phone" binding:"required,min=6,max=20"`
	Scene string `json:"scene" binding:"required"`
}

type SendPhoneCodeResponse struct {
	Action          string `json:"action"`
	CooldownSeconds int32  `json:"cooldown_seconds"`
	Message         string `json:"message"`
	DebugCode       string `json:"debug_code,omitempty"`
}

type PhoneAuthEntryRequest struct {
	Phone            string `json:"phone" binding:"required,min=6,max=20"`
	VerificationCode string `json:"verification_code" binding:"required"`
	AppCode          string `json:"app_code" binding:"required"`
	DeviceId         string `json:"device_id" binding:"required"`
}

type PhoneAuthEntryResponse struct {
	Action         string `json:"action"`
	AccessToken    string `json:"access_token,omitempty"`
	RefreshToken   string `json:"refresh_token,omitempty"`
	UserID         int64  `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	ShouldBindEmail bool  `json:"should_bind_email,omitempty"`
	Message        string `json:"message"`
}

type PhonePasswordLoginRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Password string `json:"password" binding:"required"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceId string `json:"device_id" binding:"required"`
}

type PhonePasswordLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Phone        string `json:"phone"`
	Message      string `json:"message"`
}
