package dto

// RegisterRequest is the HTTP request body for user registration.
type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=20,alphanum"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest is the HTTP request body for user login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// LogoutRequest is the HTTP request body for user logout.
type LogoutRequest struct {
	AppCode  string `json:"app_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

// RegisterResponse is the HTTP response body for user registration.
type RegisterResponse struct {
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

// LoginResponse is the HTTP response body for user login.
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

// ChangePasswordRequest is the HTTP request body for changing a password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePasswordResponse is the HTTP response body for changing a password.
type ChangePasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// LogoutAllSessionsResponse is the HTTP response body for signing out from all devices.
type LogoutAllSessionsResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

// BindEmailRequest is the HTTP request body for binding an email address.
type BindEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// BindEmailResponse is the HTTP response body for binding an email address.
type BindEmailResponse struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// SetPasswordRequest is the HTTP request body for setting a password.
type SetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// SetPasswordResponse is the HTTP response body for setting a password.
type SetPasswordResponse struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}
