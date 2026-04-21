package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// TranslateValidationError 将 validator 的错误翻译成友好的中文提示
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
		return fmt.Sprintf("%s不能为空", field)
	case "min":
		return fmt.Sprintf("%s长度必须至少为%s个字符", field, param)
	case "max":
		return fmt.Sprintf("%s长度不能超过%s个字符", field, param)
	case "email":
		return fmt.Sprintf("%s必须是有效的邮箱地址", field)
	case "alphanum":
		return fmt.Sprintf("%s只能包含字母和数字", field)
	default:
		return fmt.Sprintf("%s格式不正确", field)
	}
}
