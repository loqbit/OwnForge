package config

import (
	"os"
	"strings"

	"github.com/luckysxx/common/conf"
	commonOtel "github.com/luckysxx/common/otel"
	commonRedis "github.com/luckysxx/common/redis"
)

// Config 定义了 API Gateway 的完整配置结构。
type Config struct {
	AppEnv    string             `mapstructure:"app_env"`
	Server    ServerConfig       `mapstructure:"server"`
	Routes    RoutesConfig       `mapstructure:"routes"`
	JWT       JWTConfig          `mapstructure:"jwt"`
	Redis     commonRedis.Config `mapstructure:"redis"`
	OTel      commonOtel.Config  `mapstructure:"otel"`
	Client    ClientConfig       `mapstructure:"client"`
	SSOCookie SSOCookieConfig    `mapstructure:"sso_cookie"`
}

// ServerConfig 定义了网关监听和跨域配置。
type ServerConfig struct {
	Port        string   `mapstructure:"port"`
	CorsOrigins []string `mapstructure:"cors_origins"`
}

// RoutesConfig 定义了网关访问下游服务的地址配置。
type RoutesConfig struct {
	UserPlatformHTTP string `mapstructure:"user_platform_http"`
	UserPlatformGRPC string `mapstructure:"user_platform_grpc"`
	GoNote           string `mapstructure:"go_note"`
	GoNoteGRPC       string `mapstructure:"go_note_grpc"`
	GoChat           string `mapstructure:"go_chat"`
}

// JWTConfig 定义了网关验签使用的 JWT 配置。
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// ClientConfig 暴露给前端应用的运行时配置（公网 URL）。
// 通过 GET /api/v1/config/client 返回给浏览器，
// 替代构建时注入的 VITE_* 环境变量。
type ClientConfig struct {
	SSOLoginURL string `mapstructure:"sso_login_url" json:"sso_login_url"`
	GoNoteURL   string `mapstructure:"go_note_url" json:"go_note_url"`
	GoChatURL   string `mapstructure:"go_chat_url" json:"go_chat_url"`
}

// SSOCookieConfig 定义网关写入浏览器 SSO Cookie 的配置。
type SSOCookieConfig struct {
	Name     string `mapstructure:"name"`
	Domain   string `mapstructure:"domain"`
	Path     string `mapstructure:"path"`
	MaxAge   int    `mapstructure:"max_age"`
	Secure   bool   `mapstructure:"secure"`
	HTTPOnly bool   `mapstructure:"http_only"`
	SameSite string `mapstructure:"same_site"`
}

// LoadConfig 从配置文件和环境变量中加载网关配置。
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)

	if corsOrigins := parseListEnv("SERVER_CORS_ORIGINS"); len(corsOrigins) > 0 {
		cfg.Server.CorsOrigins = corsOrigins
	}

	return &cfg
}

func parseListEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	return values
}
