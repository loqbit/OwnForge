package config

import (
	"os"
	"strings"

	"github.com/loqbit/ownforge/pkg/conf"
	commonOtel "github.com/loqbit/ownforge/pkg/otel"
	commonRedis "github.com/loqbit/ownforge/pkg/redis"
)

// Config defines the full configuration structure for the API Gateway.
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

// ServerConfig defines gateway listen and CORS settings.
type ServerConfig struct {
	Port        string   `mapstructure:"port"`
	CorsOrigins []string `mapstructure:"cors_origins"`
}

// RoutesConfig defines downstream service addresses used by the gateway.
type RoutesConfig struct {
	UserPlatformHTTP string `mapstructure:"user_platform_http"`
	UserPlatformGRPC string `mapstructure:"user_platform_grpc"`
	GoNote           string `mapstructure:"go_note"`
	GoNoteGRPC       string `mapstructure:"go_note_grpc"`
	GoChat           string `mapstructure:"go_chat"`
}

// JWTConfig defines the JWT settings used by the gateway for verification.
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// ClientConfig exposes runtime config to frontend apps, such as public URLs.
// It is returned to the browser by GET /api/v1/config/client,
// replacing build-time VITE_* environment-variable injection.
type ClientConfig struct {
	SSOLoginURL string `mapstructure:"sso_login_url" json:"sso_login_url"`
	GoNoteURL   string `mapstructure:"go_note_url" json:"go_note_url"`
	GoChatURL   string `mapstructure:"go_chat_url" json:"go_chat_url"`
}

// SSOCookieConfig defines how the gateway writes the browser SSO cookie.
type SSOCookieConfig struct {
	Name     string `mapstructure:"name"`
	Domain   string `mapstructure:"domain"`
	Path     string `mapstructure:"path"`
	MaxAge   int    `mapstructure:"max_age"`
	Secure   bool   `mapstructure:"secure"`
	HTTPOnly bool   `mapstructure:"http_only"`
	SameSite string `mapstructure:"same_site"`
}

// LoadConfig loads gateway config from files and environment variables.
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
