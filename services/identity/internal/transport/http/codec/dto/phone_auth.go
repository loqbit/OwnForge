package dto

// SendPhoneCodeRequest is the HTTP request body for the phone verification flow.
type SendPhoneCodeRequest struct {
	Phone string `json:"phone" binding:"required,min=6,max=20"`
	Scene string `json:"scene" binding:"required"`
}

// SendPhoneCodeResponse is the HTTP response body for the phone verification flow.
type SendPhoneCodeResponse struct {
	Action          string `json:"action"`
	CooldownSeconds int    `json:"cooldown_seconds,omitempty"`
	Message         string `json:"message,omitempty"`
	DebugCode       string `json:"debug_code,omitempty"`
}

// PhoneAuthEntryRequest is the HTTP request body for the combined phone login or signup flow.
type PhoneAuthEntryRequest struct {
	Phone            string `json:"phone" binding:"required,min=6,max=20"`
	VerificationCode string `json:"verification_code" binding:"required"`
	AppCode          string `json:"app_code" binding:"required"`
	DeviceID         string `json:"device_id" binding:"required"`
}

// PhoneAuthEntryResponse is the HTTP response body for the combined phone login or signup flow.
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

// PhonePasswordLoginRequest is the HTTP request body for phone-and-password login.
type PhonePasswordLoginRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Password string `json:"password" binding:"required,min=8"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// PhonePasswordLoginResponse is the HTTP response body for phone-and-password login.
type PhonePasswordLoginResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	UserID       int64  `json:"user_id,omitempty"`
	Username     string `json:"username,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Message      string `json:"message,omitempty"`
}
