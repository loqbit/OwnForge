package authservice

// LoginCommand contains the input parameters for username-and-password login.
type LoginCommand struct {
	Username string
	Password string
	AppCode  string
	DeviceID string
}

// LogoutCommand contains the input parameters for logout.
type LogoutCommand struct {
	UserID   int64
	AppCode  string
	DeviceID string
}

// LoginResult is returned after a successful login.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
}

// VerifyTokenCommand contains the input parameters for access-token verification.
type VerifyTokenCommand struct {
	Token string
}

// VerifyTokenResult is returned after access-token verification.
type VerifyTokenResult struct {
	UserID   int64
	Username string
}

// RefreshTokenCommand contains the input parameters for refreshing a token.
type RefreshTokenCommand struct {
	Token string
}

// RefreshTokenResult is returned after refreshing a token.
type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
}

// ExchangeSSOCommand contains the input parameters for exchanging an SSO cookie for the current app session.
type ExchangeSSOCommand struct {
	SSOToken string
	AppCode  string
	DeviceID string
}

// SendPhoneCodeCommand contains the input parameters for sending a phone verification code.
type SendPhoneCodeCommand struct {
	Phone string
	Scene string
}

// SendPhoneCodeResult is returned after sending a phone verification code.
type SendPhoneCodeResult struct {
	Action          string
	CooldownSeconds int
	Message         string
	DebugCode       string
}

// PhoneAuthEntryCommand contains the input parameters for the combined phone-code login or signup flow.
type PhoneAuthEntryCommand struct {
	Phone            string
	VerificationCode string
	AppCode          string
	DeviceID         string
}

// PhoneAuthEntryResult is returned by the combined phone-code login or signup flow.
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

// PhonePasswordLoginCommand contains the input parameters for phone-and-password login.
type PhonePasswordLoginCommand struct {
	Phone    string
	Password string
	AppCode  string
	DeviceID string
}

// PhonePasswordLoginResult is returned after phone-and-password login.
type PhonePasswordLoginResult struct {
	AccessToken  string
	RefreshToken string
	SSOToken     string
	UserID       int64
	Username     string
	Phone        string
	Message      string
}
