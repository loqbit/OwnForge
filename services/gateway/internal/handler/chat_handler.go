package handler

import (
	"net/http/httputil"

	"github.com/gin-gonic/gin"
)

// ChatHandler forwards chat-related requests to the go-chat service.
type ChatHandler struct {
	proxy *httputil.ReverseProxy
}

// NewChatHandler creates a chat forwarding handler.
func NewChatHandler(proxy *httputil.ReverseProxy) *ChatHandler {
	return &ChatHandler{proxy: proxy}
}

// Proxy forwards the current request to the downstream chat service.
func (h *ChatHandler) Proxy(c *gin.Context) {
	h.proxy.ServeHTTP(c.Writer, c.Request)
}
