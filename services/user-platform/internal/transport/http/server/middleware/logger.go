package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luckysxx/common/trace"
	"go.uber.org/zap"
)

// GinLogger 返回一个记录 HTTP 请求信息的 Gin 中间件。
func GinLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 将 logger 存入 context，供 response.Error 使用。
		c.Set("logger", log)

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next() // 执行后续的逻辑

		// Docker 健康检查很频繁（每 10 秒一次），为了防止日志刷屏，这里直接过滤 /health 的日志记录。
		if path == "/health" {
			return
		}

		// 请求结束，记录日志
		cost := time.Since(start)
		status := c.Writer.Status()
		traceID := trace.FromContext(c.Request.Context())

		fields := []zap.Field{
			zap.String("trace_id", traceID),
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
			log.Error("服务器内部错误", fields...)
		} else if status >= 400 {
			log.Warn("请求异常", fields...)
		} else {
			log.Info("请求", fields...)
		}
	}
}

// GinRecovery 捕获项目运行过程中可能出现的 panic，并使用 zap 记录相关日志。
func GinRecovery(log *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 识别断开的连接，这类情况通常不需要打印完整的 panic 堆栈。
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					log.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// 如果连接已经断开，就无法继续回写响应状态码。
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					log.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					log.Error("[Recovery from panic]",
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
