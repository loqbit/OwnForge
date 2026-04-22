package errs

import (
	"fmt"
)

// CustomError 自定义错误
// 既包含给前端看的 Msg，也包含给后端查日志用的原始 Err
type CustomError struct {
	Code int    // 业务码
	Msg  string // 提示信息 (User Friendly)
	Err  error  // 原始错误 (Developer Friendly, 用于记录日志)
}

// 实现 error 接口，这样 CustomError 就可以当做普通 error 返回
func (e *CustomError) Error() string {
	// 默认打印出 Msg，如果需要打印深层错误日志，单独处理 Err 字段
	return fmt.Sprintf("Code: %d, Msg: %s, Err: %v", e.Code, e.Msg, e.Err)
}

// New 创建通用业务错误
func New(code int, msg string, err error) *CustomError {
	return &CustomError{
		Code: code,
		Msg:  msg,
		Err:  err,
	}
}

// NewParamErr 参数错误 (比如: 密码太短)
func NewParamErr(msg string, err error) *CustomError {
	return New(ParamErr, msg, err)
}

// NewServerErr 系统错误 (比如: 数据库查询失败)
// 注意：这里通常把 msg 写死为 "系统繁忙"，不让用户看到 SQL 报错
func NewServerErr(err error) *CustomError {
	return New(ServerErr, "系统繁忙，请稍后再试", err)
}
