package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// TranslateValidationError converts validator errors into user-friendly Chinese messages.
func TranslateValidationError(err error) string {
	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	var messages []string

	for _, fieldErr := range validationErrs {
		message := translateFieldError(fieldErr)
		messages = append(messages, message)
	}

	return strings.Join(messages, ", ")
}

func translateFieldError(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	tag := fieldErr.Tag()
	param := fieldErr.Param()

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
		return fmt.Sprintf("%s has an invalid format", field)
	}
}
