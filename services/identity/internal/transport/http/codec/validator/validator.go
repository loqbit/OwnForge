package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// TranslateValidationError converts validator errors into user-friendly Chinese messages.
func TranslateValidationError(err error) string {
	// Type assertion: check whether err is validator.ValidationErrors.
	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		// Return the original error when it is not a validation error.
		return err.Error()
	}

	var messages []string

	// Walk through validation errors field by field.
	for _, fieldErr := range validationErrs {
		// fieldErr.Field() is the field name, for example "Username".
		// fieldErr.Tag() is the validation rule, for example "required", "min", or "email".
		// fieldErr.Param() is the rule parameter, for example the "3" in min=3.

		message := translateFieldError(fieldErr)
		messages = append(messages, message)
	}

	// Join all error messages with commas.
	return strings.Join(messages, ", ")
}

// translateFieldError translates one field-level validation error.
func translateFieldError(fieldErr validator.FieldError) string {
	field := fieldErr.Field() // Field name.
	tag := fieldErr.Tag()     // Validation tag.
	param := fieldErr.Param() // Rule parameter.

	// Return a Chinese message based on the validation tag.
	switch tag {
	case "required":
		return fmt.Sprintf("%s cannot be empty", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", field, param)
	case "max":
		return fmt.Sprintf("%s cannot exceed %s characters", field, param)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "alphanum":
		return fmt.Sprintf("%s can only contain letters and numbers", field)
	default:
		// Fall back to a default message when no rule matches.
		return fmt.Sprintf("%s has an invalid format", field)
	}
}
