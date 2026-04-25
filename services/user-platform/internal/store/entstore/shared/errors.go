package shared

import (
	"strings"

	"github.com/loqbit/ownforge/services/user-platform/internal/ent"
	sharedrepo "github.com/loqbit/ownforge/services/user-platform/internal/repository/shared"
)

// ParseEntError converts Ent errors into repository-layer sentinel errors.
// This is the only place in the project that needs to understand Ent-specific error types.
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

	// System-level or network-level errors.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "dial tcp") {
		return sharedrepo.ErrConnection
	}

	return err
}

// parseConstraintError extracts the concrete business error from an Ent constraint error string.
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
