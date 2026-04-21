package errs

import (
	"errors"

	pkgerrs "github.com/luckysxx/common/errs"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatusError 将领域/存储层错误转换为 gRPC status error。
func ToStatusError(err error) error {
	if err == nil {
		return nil
	}

	if s, ok := status.FromError(err); ok {
		return s.Err()
	}

	var customErr *pkgerrs.CustomError
	if errors.As(err, &customErr) {
		return status.Error(toGRPCCode(customErr.Code), customErr.Msg)
	}

	switch {
	case errors.Is(err, sharedrepo.ErrDuplicateKey):
		return status.Error(codes.AlreadyExists, "记录已存在")
	case errors.Is(err, sharedrepo.ErrNoRows):
		return status.Error(codes.NotFound, "记录不存在")
	case errors.Is(err, sharedrepo.ErrForeignKey):
		return status.Error(codes.FailedPrecondition, "关联记录不存在")
	case errors.Is(err, sharedrepo.ErrCheckViolation),
		errors.Is(err, sharedrepo.ErrNotNullViolation),
		errors.Is(err, sharedrepo.ErrInvalidData):
		return status.Error(codes.InvalidArgument, "请求数据不合法")
	case errors.Is(err, sharedrepo.ErrConnection):
		return status.Error(codes.Unavailable, "数据库连接失败，请稍后重试")
	default:
		return status.Error(codes.Internal, "系统繁忙，请稍后再试")
	}
}

func toGRPCCode(code int) codes.Code {
	switch code {
	case pkgerrs.ParamErr:
		return codes.InvalidArgument
	case pkgerrs.Unauthorized:
		return codes.Unauthenticated
	case pkgerrs.Forbidden:
		return codes.PermissionDenied
	case pkgerrs.NotFound:
		return codes.NotFound
	default:
		return codes.Internal
	}
}
