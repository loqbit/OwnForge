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

// GinLogger 返回一个记录 HTTP 请求信息的 Gin 中间件。
// 自动从 context 中提取 OTel TraceID 注入到每条请求日志中。
//
// 使用方式（在 router.go 或 main.go 中）：
//
//	r.Use(logger.GinLogger(log))
func GinLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 将 logger 存储到 gin.Context 中，供下游 handler（如 response.Error）使用
		c.Set("logger", log)

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next() // 执行后续逻辑

		// Docker 健康检查很频繁，过滤掉避免刷屏
		if path == "/health" || path == "/metrics" {
			return
		}

		cost := time.Since(start)
		status := c.Writer.Status()

		// 从 context 提取 TraceID，自动附加到日志
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
			reqLog.Error("服务器内部错误", fields...)
		} else if status >= 400 {
			reqLog.Warn("请求异常", fields...)
		} else {
			reqLog.Info("请求", fields...)
		}
	}
}

// GinRecovery 捕获 panic 并使用 zap 记录堆栈信息，防止服务崩溃。
//
// 使用方式：
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

				// 从 context 提取 TraceID
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
