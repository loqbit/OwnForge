package errs

import (
	"errors"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
)

// ConvertToCustomError 将领域层和存储层错误转换为面向 HTTP 的业务错误。
func ConvertToCustomError(err error) *pkgerrs.CustomError {
	if err == nil {
		return nil
	}

	var customErr *pkgerrs.CustomError
	if errors.As(err, &customErr) {
		return customErr
	}

	switch {
	case errors.Is(err, sharedrepo.ErrUsernameDuplicate):
		return pkgerrs.NewParamErr("用户名已存在", err)
	case errors.Is(err, sharedrepo.ErrEmailDuplicate):
		return pkgerrs.NewParamErr("邮箱已存在", err)
	case errors.Is(err, sharedrepo.ErrDuplicateKey):
		return pkgerrs.NewParamErr("记录已存在", err)
	case errors.Is(err, sharedrepo.ErrNoRows):
		return pkgerrs.New(pkgerrs.NotFound, "记录不存在", err)
	case errors.Is(err, sharedrepo.ErrForeignKey):
		return pkgerrs.NewParamErr("关联记录不存在", err)
	case errors.Is(err, sharedrepo.ErrCheckViolation):
		return pkgerrs.NewParamErr("数据校验失败", err)
	case errors.Is(err, sharedrepo.ErrNotNullViolation):
		return pkgerrs.NewParamErr("必填字段缺失", err)
	case errors.Is(err, sharedrepo.ErrInvalidData):
		return pkgerrs.NewParamErr("数据格式错误", err)
	case errors.Is(err, sharedrepo.ErrConnection):
		return pkgerrs.New(pkgerrs.ServerErr, "数据库连接失败，请稍后重试", err)
	}

	return pkgerrs.NewServerErr(err)
}
