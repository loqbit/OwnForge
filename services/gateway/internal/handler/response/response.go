package response

import (
	"errors"
	"net/http"

	"github.com/luckysxx/common/errs"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Success 统一成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": errs.Success,
		"msg":  "success",
		"data": data,
	})
}

// Error 统一错误响应
func Error(c *gin.Context, err error) {
	// 获取logger（如果存在）
	logger, exists := c.Get("logger")
	var zapLogger *zap.Logger
	if exists {
		zapLogger, _ = logger.(*zap.Logger)
	}

	var customErr *errs.CustomError
	if errors.As(err, &customErr) {
		// 记录详细的错误信息到日志
		if zapLogger != nil {
			// 记录详细错误（包含原始错误）
			if customErr.Err != nil {
				zapLogger.Error("业务错误",
					zap.Int("code", customErr.Code),
					zap.String("msg", customErr.Msg),
					zap.Error(customErr.Err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
			} else {
				zapLogger.Warn("业务错误",
					zap.Int("code", customErr.Code),
					zap.String("msg", customErr.Msg),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"code": customErr.Code,
			"msg":  customErr.Msg,
			"data": nil,
		})
		return
	}

	// 未知错误 - 记录完整的错误信息
	if zapLogger != nil {
		zapLogger.Error("未知错误",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("ip", c.ClientIP()),
		)
	}
	_ = c.Error(err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"code": errs.ServerErr,
		"msg":  "系统繁忙",
		"data": nil,
	})
}

// BadRequest 参数错误响应
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code": errs.ParamErr,
		"msg":  msg,
		"data": nil,
	})
}

// NotFound 资源未找到响应
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{
		"code": errs.NotFound,
		"msg":  msg,
		"data": nil,
	})
}

// Unauthorized 未授权响应
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"code": errs.Unauthorized,
		"msg":  msg,
		"data": nil,
	})
}

// Forbidden 禁止访问响应
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{
		"code": errs.Forbidden,
		"msg":  msg,
		"data": nil,
	})
}
