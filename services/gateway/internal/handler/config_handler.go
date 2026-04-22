package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ownforge/ownforge/services/gateway/internal/config"
	"github.com/ownforge/ownforge/services/gateway/internal/handler/response"
)

// ConfigHandler handles frontend runtime-config requests.
type ConfigHandler struct {
	clientCfg config.ClientConfig
}

// NewConfigHandler creates a ConfigHandler with preloaded client config.
func NewConfigHandler(clientCfg config.ClientConfig) *ConfigHandler {
	return &ConfigHandler{clientCfg: clientCfg}
}

// GetClientConfig returns the runtime config required by the frontend. It is public.
// Response example:
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
