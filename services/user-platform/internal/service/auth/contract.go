package authservice

// LoginCommand 表示用户名密码登录时的输入参数。
type LoginCommand struct {
	Username string
	Password string
	AppCode  string
	DeviceID string
}

// LogoutCommand 表示登出时的输入参数。
type LogoutCommand struct {
	UserID   int64
	AppCode  string
	DeviceID string
}

// LoginResult 表示登录成功后的返回结果。
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
}

// VerifyTokenCommand 表示访问令牌校验时的输入参数。
type VerifyTokenCommand struct {
	Token string
}

// VerifyTokenResult 表示访问令牌校验后的结果。
type VerifyTokenResult struct {
	UserID   int64
	Username string
}

// RefreshTokenCommand 表示刷新令牌时的输入参数。
type RefreshTokenCommand struct {
	Token string
}

// RefreshTokenResult 表示刷新令牌后的返回结果。
type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
}

// ExchangeSSOCommand 表示通过 SSO Cookie 换取当前应用会话时的输入参数。
type ExchangeSSOCommand struct {
	SSOToken string
	AppCode  string
	DeviceID string
}

// SendPhoneCodeCommand 表示发送手机验证码时的输入参数。
type SendPhoneCodeCommand struct {
	Phone string
	Scene string
}

// SendPhoneCodeResult 表示发送手机验证码后的结果。
type SendPhoneCodeResult struct {
	Action          string
	CooldownSeconds int
	Message         string
	DebugCode       string
}

// PhoneAuthEntryCommand 表示手机号验证码登录或注册一体化流程的输入参数。
type PhoneAuthEntryCommand struct {
	Phone            string
	VerificationCode string
	AppCode          string
	DeviceID         string
}

// PhoneAuthEntryResult 表示手机号验证码登录或注册一体化流程的结果。
type PhoneAuthEntryResult struct {
	Action          string
	AccessToken     string
	RefreshToken    string
	SSOToken        string
	UserID          int64
	Username        string
	Email           string
	Phone           string
	ShouldBindEmail bool
	Message         string
}

// PhonePasswordLoginCommand 表示手机号密码登录时的输入参数。
type PhonePasswordLoginCommand struct {
	Phone    string
	Password string
	AppCode  string
	DeviceID string
}

// PhonePasswordLoginResult 表示手机号密码登录后的返回结果。
type PhonePasswordLoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
	Phone        string
	Message      string
}
