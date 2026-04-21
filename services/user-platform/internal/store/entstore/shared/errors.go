package shared

import (
	"strings"

	"github.com/ownforge/ownforge/services/user-platform/internal/ent"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
)

// ParseEntError 将 Ent 框架的错误转为 repository 层的哨兵错误。
// 这是整个项目中唯一需要理解 ent 错误类型的地方。
func ParseEntError(err error) error {
	if err == nil {
		return nil
	}

	if ent.IsNotFound(err) {
		return sharedrepo.ErrNoRows
	}
	if ent.IsValidationError(err) {
		return sharedrepo.ErrInvalidData
	}
	if ent.IsConstraintError(err) {
		return parseConstraintError(err)
	}

	// 系统级/网络级错误
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "dial tcp") {
		return sharedrepo.ErrConnection
	}

	return err
}

// parseConstraintError 从 ent 约束错误文本中提取具体业务错误。
func parseConstraintError(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "users_phone_key"):
		return sharedrepo.ErrPhoneDuplicate
	case strings.Contains(msg, "users_username_key"):
		return sharedrepo.ErrUsernameDuplicate
	case strings.Contains(msg, "users_email_key"):
		return sharedrepo.ErrEmailDuplicate
	case strings.Contains(msg, "duplicate key"),
		strings.Contains(msg, "unique constraint"),
		strings.Contains(msg, "already exists"):
		return sharedrepo.ErrDuplicateKey
	case strings.Contains(msg, "foreign key"):
		return sharedrepo.ErrForeignKey
	case strings.Contains(msg, "not-null"),
		strings.Contains(msg, "not null"):
		return sharedrepo.ErrNotNullViolation
	case strings.Contains(msg, "check constraint"):
		return sharedrepo.ErrCheckViolation
	default:
		return sharedrepo.ErrConstraint
	}
}
