package errs

import (
	"errors"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCError 将领域层和存储层错误转换为 gRPC 状态错误。
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	var customErr *pkgerrs.CustomError
	if errors.As(err, &customErr) {
		switch customErr.Code {
		case pkgerrs.ParamErr:
			return status.Error(codes.InvalidArgument, customErr.Msg)
		case pkgerrs.Unauthorized:
			return status.Error(codes.Unauthenticated, customErr.Msg)
		case pkgerrs.Forbidden:
			return status.Error(codes.PermissionDenied, customErr.Msg)
		case pkgerrs.NotFound:
			return status.Error(codes.NotFound, customErr.Msg)
		default:
			return status.Error(codes.Internal, customErr.Msg)
		}
	}

	switch {
	case errors.Is(err, sharedrepo.ErrInvalidOrExpiredToken):
		return status.Error(codes.Unauthenticated, "无效的刷新凭证或已过期")
	case errors.Is(err, sharedrepo.ErrUsernameDuplicate),
		errors.Is(err, sharedrepo.ErrEmailDuplicate),
		errors.Is(err, sharedrepo.ErrDuplicateKey),
		errors.Is(err, sharedrepo.ErrForeignKey),
		errors.Is(err, sharedrepo.ErrCheckViolation),
		errors.Is(err, sharedrepo.ErrNotNullViolation),
		errors.Is(err, sharedrepo.ErrInvalidData):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, sharedrepo.ErrNoRows):
		return status.Error(codes.NotFound, "记录不存在")
	case errors.Is(err, sharedrepo.ErrConnection):
		return status.Error(codes.Unavailable, "数据库连接失败，请稍后重试")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
