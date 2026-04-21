package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/luckysxx/common/trace"
)

// TraceMiddleware 注入和提取全链路追踪 ID
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 尝试从 Header 中获取前置服务/网关传来的 Trace ID
		traceID := c.GetHeader(trace.HeaderTraceID)
		if traceID == "" {
			// 2. 如果没有，则生成一个新的
			traceID = trace.NewTraceID()
		}

		// 3. 将 Trace ID 塞入当前请求的 Header 中（方便前端或其他后续系统拿到）
		c.Header(trace.HeaderTraceID, traceID)

		// 4. 将 Trace ID 塞进基于标准库的 Context，这样后续 Service / DB 可以通过 ctx.Request 获取它
		ctx := trace.IntoContext(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
