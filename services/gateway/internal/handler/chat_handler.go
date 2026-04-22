package handler

import (
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

// ChatHandler 负责将聊天相关请求转发到 go-chat 服务。
type ChatHandler struct {
	proxy *httputil.ReverseProxy
}

// NewChatHandler 创建一个聊天转发 Handler。
func NewChatHandler(proxy *httputil.ReverseProxy) *ChatHandler {
	return &ChatHandler{proxy: proxy}
}

// Proxy 将当前请求透传给下游聊天服务。
func (h *ChatHandler) Proxy(c *gin.Context) {
	h.proxy.ServeHTTP(c.Writer, c.Request)
}
