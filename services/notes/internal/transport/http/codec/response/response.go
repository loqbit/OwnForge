package response

import (
	"errors"
	"net/http"

	"github.com/loqbit/ownforge/pkg/errs"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Success writes the standard success response.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": errs.Success,
		"msg":  "success",
		"data": data,
	})
}

// Error writes the standard error response.
func Error(c *gin.Context, err error) {
	// Reuse the request logger when available.
	logger, exists := c.Get("logger")
	var zapLogger *zap.Logger
	if exists {
		zapLogger, _ = logger.(*zap.Logger)
	}

	var customErr *errs.CustomError
	if errors.As(err, &customErr) {
		if zapLogger != nil {
			if customErr.Err != nil {
				zapLogger.Error("business error",
					zap.Int("code", customErr.Code),
					zap.String("msg", customErr.Msg),
					zap.Error(customErr.Err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
			} else {
				zapLogger.Warn("business error",
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

	// Unknown error.
	if zapLogger != nil {
		zapLogger.Error("unknown error",
			zap.Error(err),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("ip", c.ClientIP()),
		)
	}
	_ = c.Error(err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"code": errs.ServerErr,
		"msg":  "system busy",
		"data": nil,
	})
}

// BadRequest writes a parameter error response.
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code": errs.ParamErr,
		"msg":  msg,
		"data": nil,
	})
}

// NotFound writes a resource-not-found response.
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{
		"code": errs.NotFound,
		"msg":  msg,
		"data": nil,
	})
}

// Unauthorized writes an unauthorized response.
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"code": errs.Unauthorized,
		"msg":  msg,
		"data": nil,
	})
}

// Forbidden writes a forbidden response.
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{
		"code": errs.Forbidden,
		"msg":  msg,
		"data": nil,
	})
}
