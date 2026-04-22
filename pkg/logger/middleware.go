package logger

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinLogger returns a Gin middleware that records HTTP request information.。
// Automatically extract the OTel TraceID from context and inject it into every request log.
//
// Usage (in router.go or main.go):
//
//	r.Use(logger.GinLogger(log))
func GinLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Store the logger in gin.Context so downstream handlers such as response.Error can use it.
		c.Set("logger", log)

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next() // execute downstream logic

		// Docker health checks are frequent, so filter them out to avoid spam.
		if path == "/health" || path == "/metrics" {
			return
		}

		cost := time.Since(start)
		status := c.Writer.Status()

		// Extract the TraceID from context and attach it to logs automatically.
		reqLog := Ctx(c.Request.Context(), log)

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.Duration("cost", cost),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		if status >= 500 {
			reqLog.Error("internal server error", fields...)
		} else if status >= 400 {
			reqLog.Warn("request error", fields...)
		} else {
			reqLog.Info("request", fields...)
		}
	}
}

// GinRecovery catches panics and records the stacktrace with zap to prevent service crashes.
//
// Usage:
//
//	r.Use(logger.GinRecovery(log, true))
func GinRecovery(log *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)

				// extract TraceID from context
				reqLog := Ctx(c.Request.Context(), log)

				if brokenPipe {
					reqLog.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					c.Error(err.(error))
					c.Abort()
					return
				}

				if stack {
					reqLog.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					reqLog.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
