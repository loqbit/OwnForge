package response

import (
	"errors"
	"net/http"

	"github.com/ownforge/ownforge/pkg/errs"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Success sends the standard success response.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": errs.Success,
		"msg":  "success",
		"data": data,
	})
}

// Error sends the standard error response.
func Error(c *gin.Context, err error) {
	// Fetch the logger if present.
	logger, exists := c.Get("logger")
	var zapLogger *zap.Logger
	if exists {
		zapLogger, _ = logger.(*zap.Logger)
	}

	var customErr *errs.CustomError
	if errors.As(err, &customErr) {
		// Record detailed error information in the logs.
		if zapLogger != nil {
			// Record detailed errors including the original error.
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

	// unknown error: record the full error details
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

// BadRequest parametererror response
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code": errs.ParamErr,
		"msg":  msg,
		"data": nil,
	})
}

// NotFound sends a resource-not-found response.
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, gin.H{
		"code": errs.NotFound,
		"msg":  msg,
		"data": nil,
	})
}

// Unauthorized sends an unauthorized response
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"code": errs.Unauthorized,
		"msg":  msg,
		"data": nil,
	})
}

// Forbidden sends a forbidden response.
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{
		"code": errs.Forbidden,
		"msg":  msg,
		"data": nil,
	})
}
