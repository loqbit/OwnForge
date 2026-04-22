package sharedrepo

import "errors"

// Sentinel errors for database operations.
var (
	ErrNoRows           = errors.New("record not found")
	ErrDuplicateKey     = errors.New("record already exists")
	ErrForeignKey       = errors.New("related record not found")
	ErrCheckViolation   = errors.New("data validation failed")
	ErrNotNullViolation = errors.New("required field missing")
	ErrInvalidData      = errors.New("invalid data format")
	ErrConnection       = errors.New("database connection failed")
	ErrConstraint       = errors.New("data constraint violated")

	ErrPhoneDuplicate    = errors.New("phone number already exists")
	ErrUsernameDuplicate = errors.New("username already exists")
	ErrEmailDuplicate    = errors.New("email already exists")

	ErrInvalidOrExpiredToken = errors.New("invalid or expired token")
)

// IsNotFoundError reports whether the error means the record does not exist.
func IsNotFoundError(err error) bool {
	return err != nil && errors.Is(err, ErrNoRows)
}

// IsDuplicateKeyError reports whether the error means a unique-key conflict.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrDuplicateKey) ||
		errors.Is(err, ErrPhoneDuplicate) ||
		errors.Is(err, ErrUsernameDuplicate) ||
		errors.Is(err, ErrEmailDuplicate)
}

// IsForeignKeyError reports whether the error means a foreign-key constraint violation.
func IsForeignKeyError(err error) bool {
	return err != nil && errors.Is(err, ErrForeignKey)
}

// IsConnectionError reports whether the error means a database connection failure.
func IsConnectionError(err error) bool {
	return err != nil && errors.Is(err, ErrConnection)
}
