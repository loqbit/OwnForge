package errs

import (
	"errors"

	pkgerrs "github.com/loqbit/ownforge/pkg/errs"
	sharedrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCError converts domain and storage errors into gRPC status errors.
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
		return status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	case errors.Is(err, sharedrepo.ErrUsernameDuplicate),
		errors.Is(err, sharedrepo.ErrEmailDuplicate),
		errors.Is(err, sharedrepo.ErrDuplicateKey),
		errors.Is(err, sharedrepo.ErrForeignKey),
		errors.Is(err, sharedrepo.ErrCheckViolation),
		errors.Is(err, sharedrepo.ErrNotNullViolation),
		errors.Is(err, sharedrepo.ErrInvalidData):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, sharedrepo.ErrNoRows):
		return status.Error(codes.NotFound, "record not found")
	case errors.Is(err, sharedrepo.ErrConnection):
		return status.Error(codes.Unavailable, "database connection failed, please try again later")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
