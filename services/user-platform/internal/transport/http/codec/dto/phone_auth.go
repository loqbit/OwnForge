package dto

// SendPhoneCodeRequest 表示手机号验证码流程的 HTTP 请求体。
type SendPhoneCodeRequest struct {
	Phone string `json:"phone" binding:"required,min=6,max=20"`
	Scene string `json:"scene" binding:"required"`
}

// SendPhoneCodeResponse 表示手机号验证码流程的 HTTP 响应体。
type SendPhoneCodeResponse struct {
	Action          string `json:"action"`
	CooldownSeconds int    `json:"cooldown_seconds,omitempty"`
	Message         string `json:"message,omitempty"`
	DebugCode       string `json:"debug_code,omitempty"`
}

// PhoneAuthEntryRequest 表示手机号登录或注册一体化流程的 HTTP 请求体。
type PhoneAuthEntryRequest struct {
	Phone            string `json:"phone" binding:"required,min=6,max=20"`
	VerificationCode string `json:"verification_code" binding:"required"`
	AppCode          string `json:"app_code" binding:"required"`
	DeviceID         string `json:"device_id" binding:"required"`
}

// PhoneAuthEntryResponse 表示手机号登录或注册一体化流程的 HTTP 响应体。
type PhoneAuthEntryResponse struct {
	Action          string `json:"action"`
	AccessToken     string `json:"access_token,omitempty"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	UserID          int64  `json:"user_id,omitempty"`
	Username        string `json:"username,omitempty"`
	Email           string `json:"email,omitempty"`
	Phone           string `json:"phone,omitempty"`
	ShouldBindEmail bool   `json:"should_bind_email,omitempty"`
	Message         string `json:"message,omitempty"`
}

// PhonePasswordLoginRequest 表示手机号加密码登录的 HTTP 请求体。
type PhonePasswordLoginRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Password string `json:"password" binding:"required,min=8"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// PhonePasswordLoginResponse 表示手机号加密码登录的 HTTP 响应体。
type PhonePasswordLoginResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	UserID       int64  `json:"user_id,omitempty"`
	Username     string `json:"username,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Message      string `json:"message,omitempty"`
}
