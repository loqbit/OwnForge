package accountservice

// RegisterCommand 表示用户注册时的输入参数。
type RegisterCommand struct {
	Phone    string
	Email    string
	Username string
	Password string
}

// RegisterResult 表示用户注册后的返回结果。
type RegisterResult struct {
	Phone    string
	Email    string
	UserID   int64
	Username string
}

// ChangePasswordCommand 表示修改密码时的输入参数。
type ChangePasswordCommand struct {
	UserID      int64
	OldPassword string
	NewPassword string
}

// ChangePasswordResult 表示修改密码后的返回结果。
type ChangePasswordResult struct {
	UserID  int64
	Message string
}

// LogoutAllSessionsCommand 表示退出全部设备时的输入参数。
type LogoutAllSessionsCommand struct {
	UserID int64
}

// LogoutAllSessionsResult 表示退出全部设备后的返回结果。
type LogoutAllSessionsResult struct {
	UserID  int64
	Message string
}

// BindEmailCommand 表示绑定邮箱时的输入参数。
type BindEmailCommand struct {
	UserID int64
	Email  string
}

// BindEmailResult 表示绑定邮箱后的返回结果。
type BindEmailResult struct {
	UserID  int64
	Email   string
	Message string
}

// SetPasswordCommand 表示设置密码时的输入参数。
type SetPasswordCommand struct {
	UserID      int64
	NewPassword string
}

// SetPasswordResult 表示设置密码后的返回结果。
type SetPasswordResult struct {
	UserID  int64
	Message string
}

// GetProfileQuery 表示查询用户资料时的输入参数。
type GetProfileQuery struct {
	UserID int64
}

// GetProfileResult 表示查询用户资料时的返回结果。
type GetProfileResult struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt string
}

// UpdateProfileCommand 表示更新用户资料时的输入参数。
type UpdateProfileCommand struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
}

// UpdateProfileResult 表示更新用户资料后的返回结果。
type UpdateProfileResult struct {
	UserID    int64
	Nickname  string
	AvatarURL string
	Bio       string
	Birthday  string
	UpdatedAt string
}
