package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/luckysxx/common/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TranslateValidationError 将 validator 的错误翻译成友好的中文提示
func TranslateValidationError(err error) string {
	// 类型断言：判断 err 是否是 validator.ValidationErrors 类型
	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		// 如果不是验证错误，返回原始错误信息
		return err.Error()
	}

	var messages []string

	// 遍历每个字段的错误
	for _, fieldErr := range validationErrs {
		// fieldErr.Field() 是字段名，如 "Username"
		// fieldErr.Tag() 是验证规则，如 "required", "min", "email"
		// fieldErr.Param() 是规则参数，如 min=3 中的 "3"

		message := translateFieldError(fieldErr)
		messages = append(messages, message)
	}

	// 用逗号连接所有错误信息
	return strings.Join(messages, ", ")
}

// translateFieldError 翻译单个字段的错误
func translateFieldError(fieldErr validator.FieldError) string {
	field := fieldErr.Field() // 字段名
	tag := fieldErr.Tag()     // 验证标签
	param := fieldErr.Param() // 参数

	// 根据不同的验证标签返回不同的中文提示
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
		// 如果没有匹配的规则，返回默认提示
		return fmt.Sprintf("%s格式不正确", field)
	}
}

// 将 gRPC 错误转换为 HTTP 错误
func ConvertToHTTPError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		// 不是 gRPC 错误，返回通用服务端错误
		return errs.New(errs.ServerErr, "系统繁忙", err)
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
