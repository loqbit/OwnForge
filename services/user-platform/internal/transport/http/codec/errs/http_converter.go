package errs

import (
	"errors"

	pkgerrs "github.com/ownforge/ownforge/pkg/errs"
	sharedrepo "github.com/ownforge/ownforge/services/user-platform/internal/repository/shared"
)

// ConvertToCustomError converts domain or storage errors into HTTP-facing business errors.
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
		return pkgerrs.NewParamErr("username already exists", err)
	case errors.Is(err, sharedrepo.ErrEmailDuplicate):
		return pkgerrs.NewParamErr("email already exists", err)
	case errors.Is(err, sharedrepo.ErrDuplicateKey):
		return pkgerrs.NewParamErr("record already exists", err)
	case errors.Is(err, sharedrepo.ErrNoRows):
		return pkgerrs.New(pkgerrs.NotFound, "record not found", err)
	case errors.Is(err, sharedrepo.ErrForeignKey):
		return pkgerrs.NewParamErr("related record not found", err)
	case errors.Is(err, sharedrepo.ErrCheckViolation):
		return pkgerrs.NewParamErr("data validation failed", err)
	case errors.Is(err, sharedrepo.ErrNotNullViolation):
		return pkgerrs.NewParamErr("required field missing", err)
	case errors.Is(err, sharedrepo.ErrInvalidData):
		return pkgerrs.NewParamErr("invalid data format", err)
	case errors.Is(err, sharedrepo.ErrConnection):
		return pkgerrs.New(pkgerrs.ServerErr, "database connection failed, please try again later", err)
	}

	return pkgerrs.NewServerErr(err)
}
