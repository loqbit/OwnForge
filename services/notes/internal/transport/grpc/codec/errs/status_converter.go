package errs

import (
	"errors"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	sharedrepo "github.com/ownforge/ownforge/services/notes/internal/repository/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatusError converts domain and storage errors into gRPC status errors.
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
		return status.Error(codes.AlreadyExists, "record already exists")
	case errors.Is(err, sharedrepo.ErrNoRows):
		return status.Error(codes.NotFound, "record not found")
	case errors.Is(err, sharedrepo.ErrForeignKey):
		return status.Error(codes.FailedPrecondition, "related record not found")
	case errors.Is(err, sharedrepo.ErrCheckViolation),
		errors.Is(err, sharedrepo.ErrNotNullViolation),
		errors.Is(err, sharedrepo.ErrInvalidData):
		return status.Error(codes.InvalidArgument, "invalid request data")
	case errors.Is(err, sharedrepo.ErrConnection):
		return status.Error(codes.Unavailable, "database connection failed, please try again later")
	default:
		return status.Error(codes.Internal, "system busy, please try again later")
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
