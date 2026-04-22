package accountservice

// RegisterCommand contains the input parameters for user registration.
type RegisterCommand struct {
	Phone    string
	Email    string
	Username string
	Password string
}

// RegisterResult is returned after user registration.
type RegisterResult struct {
	Phone    string
	Email    string
	UserID   int64
	Username string
}

// ChangePasswordCommand contains the input parameters for changing a password.
type ChangePasswordCommand struct {
	UserID      int64
	OldPassword string
	NewPassword string
}

// ChangePasswordResult is returned after a password change.
type ChangePasswordResult struct {
	UserID  int64
	Message string
}

// LogoutAllSessionsCommand contains the input parameters for signing out from all devices.
type LogoutAllSessionsCommand struct {
	UserID int64
}

// LogoutAllSessionsResult is returned after signing out from all devices.
type LogoutAllSessionsResult struct {
	UserID  int64
	Message string
}

// BindEmailCommand contains the input parameters for binding an email address.
type BindEmailCommand struct {
	UserID int64
	Email  string
}

// BindEmailResult is returned after binding an email address.
type BindEmailResult struct {
	UserID  int64
	Email   string
	Message string
}

// SetPasswordCommand contains the input parameters for setting a password.
type SetPasswordCommand struct {
	UserID      int64
	NewPassword string
}

// SetPasswordResult is returned after setting a password.
type SetPasswordResult struct {
	UserID  int64
	Message string
}

// GetProfileQuery contains the input parameters for fetching user profile data.
type GetProfileQuery struct {
	UserID int64
}

// GetProfileResult is returned when fetching user profile data.
type GetProfileResult struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt string
}

// UpdateProfileCommand contains the input parameters for updating user profile data.
type UpdateProfileCommand struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
}

// UpdateProfileResult is returned after updating user profile data.
type UpdateProfileResult struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt string
}
