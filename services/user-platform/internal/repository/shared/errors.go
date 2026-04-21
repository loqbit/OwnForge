package sharedrepo

import "errors"

// 数据库操作相关哨兵错误。
var (
	ErrNoRows           = errors.New("记录不存在")
	ErrDuplicateKey     = errors.New("记录已存在")
	ErrForeignKey       = errors.New("关联记录不存在")
	ErrCheckViolation   = errors.New("数据校验失败")
	ErrNotNullViolation = errors.New("必填字段缺失")
	ErrInvalidData      = errors.New("数据格式错误")
	ErrConnection       = errors.New("数据库连接失败")
	ErrConstraint       = errors.New("违反数据约束")

	ErrPhoneDuplicate    = errors.New("手机号已存在")
	ErrUsernameDuplicate = errors.New("用户名已存在")
	ErrEmailDuplicate    = errors.New("邮箱已存在")

	ErrInvalidOrExpiredToken = errors.New("invalid or expired token")
)

// IsNotFoundError 判断是否为记录不存在错误。
func IsNotFoundError(err error) bool {
	return err != nil && errors.Is(err, ErrNoRows)
}

// IsDuplicateKeyError 判断是否为唯一键冲突错误。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrDuplicateKey) ||
		errors.Is(err, ErrPhoneDuplicate) ||
		errors.Is(err, ErrUsernameDuplicate) ||
		errors.Is(err, ErrEmailDuplicate)
}

// IsForeignKeyError 判断是否为外键约束错误。
func IsForeignKeyError(err error) bool {
	return err != nil && errors.Is(err, ErrForeignKey)
}

// IsConnectionError 判断是否为数据库连接错误。
func IsConnectionError(err error) bool {
	return err != nil && errors.Is(err, ErrConnection)
}
