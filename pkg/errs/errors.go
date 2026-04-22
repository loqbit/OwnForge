package errs

import (
	"fmt"
)

// CustomError is the custom error type.
// It includes both the user-facing Msg and the original Err used for backend logging.
type CustomError struct {
	Code int    // business code
	Msg  string // user-facing message
	Err  error  // original error (developer-facing, used for logging)
}

// Implement the error interface so CustomError can be returned as a normal error.
func (e *CustomError) Error() string {
	// Print Msg by default; handle Err separately when detailed logs are needed.
	return fmt.Sprintf("Code: %d, Msg: %s, Err: %v", e.Code, e.Msg, e.Err)
}

// New creates a generic business error.
func New(code int, msg string, err error) *CustomError {
	return &CustomError{
		Code: code,
		Msg:  msg,
		Err:  err,
	}
}

// NewParamErr creates a parameter error, for example when a password is too short.
func NewParamErr(msg string, err error) *CustomError {
	return New(ParamErr, msg, err)
}

// NewServerErr creates a system error, for example when a database query fails.
// Note: the msg is usually fixed to "system busy" so SQL errors are not exposed to users.
func NewServerErr(err error) *CustomError {
	return New(ServerErr, "system busy, please try again later", err)
}
