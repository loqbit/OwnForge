package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/ownforge/ownforge/pkg/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TranslateValidationError converts validator errors into user-friendly messages.
func TranslateValidationError(err error) string {
	// Type assertion: check whether err is validator.ValidationErrors.
	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		// If it is not a validation error, return the original error message.
		return err.Error()
	}

	var messages []string

	// Iterate over field errors.
	for _, fieldErr := range validationErrs {
		// fieldErr.Field() is the field name, such as "Username"
		// fieldErr.Tag() is the validation rule, such as "required", "min", or "email"
		// fieldErr.Param() is the rule parameter, such as "3" in min=3

		message := translateFieldError(fieldErr)
		messages = append(messages, message)
	}

	// Join all error messages with commas.
	return strings.Join(messages, ", ")
}

// translateFieldError translates an error for a single field.
func translateFieldError(fieldErr validator.FieldError) string {
	field := fieldErr.Field() // field name
	tag := fieldErr.Tag()     // validation tag
	param := fieldErr.Param() // parameter

	// Return different user-facing messages for different validation tags.
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
		// If no rule matches, return the default message.
		return fmt.Sprintf("%s has an invalid format", field)
	}
}

// Convert a gRPC error into an HTTP error.
func ConvertToHTTPError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		// If it is not a gRPC error, return a generic server error.
		return errs.New(errs.ServerErr, "system busy", err)
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return errs.New(errs.ParamErr, st.Message(), err)
	case codes.NotFound:
		return errs.New(errs.NotFound, st.Message(), err)
	case codes.Unauthenticated:
		return errs.New(errs.Unauthorized, st.Message(), err)
	case codes.PermissionDenied:
		return errs.New(errs.Forbidden, st.Message(), err)
	default:
		return errs.New(errs.ServerErr, st.Message(), err)
	}
}
