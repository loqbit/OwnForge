package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ownforge/ownforge/services/gateway/internal/config"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
)

// ConfigHandler 处理前端运行时配置请求。
type ConfigHandler struct {
	clientCfg config.ClientConfig
}

// NewConfigHandler 创建 ConfigHandler，接收预加载的客户端配置。
func NewConfigHandler(clientCfg config.ClientConfig) *ConfigHandler {
	return &ConfigHandler{clientCfg: clientCfg}
}

// GetClientConfig 返回前端所需的运行时配置（公开接口，无需鉴权）。
// 响应示例:
//
//	{
//	  "code": 200,
//	  "msg": "success",
//	  "data": {
//	    "sso_login_url": "https://app.luckys-dev.com/auth/login",
//	    "go_note_url": "https://note.luckys-dev.com",
//	    "go_chat_url": "https://app.luckys-dev.com"
//	  }
//	}
func (h *ConfigHandler) GetClientConfig(c *gin.Context) {
	response.Success(c, h.clientCfg)
}
