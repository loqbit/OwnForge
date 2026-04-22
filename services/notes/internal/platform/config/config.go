package config

import (
	"github.com/ownforge/ownforge/pkg/conf"
	commonOtel "github.com/ownforge/ownforge/pkg/otel"
	"github.com/ownforge/ownforge/pkg/postgres"
	commonRedis "github.com/ownforge/ownforge/pkg/redis"
)

// MinIOConfig MinIO 连接配置。
type MinIOConfig struct {
	Endpoint       string   `mapstructure:"endpoint"`        // 内网连接地址 (Docker: global-minio:9000)
	PublicEndpoint string   `mapstructure:"public_endpoint"` // 浏览器可访问地址 (localhost:9000)
	AccessKey      string   `mapstructure:"access_key"`
	SecretKey      string   `mapstructure:"secret_key"`
	Bucket         string   `mapstructure:"bucket"`
	UseSSL         bool     `mapstructure:"use_ssl"`
	PresignExpiry  int      `mapstructure:"presign_expiry"`
	MaxUploadSize  int64    `mapstructure:"max_upload_size"`
	AllowedMIMEs   []string `mapstructure:"allowed_mime_types"`
}

// Config 全局配置
type Config struct {
	AppEnv      string             `mapstructure:"app_env"`
	Server      conf.ServerConfig  `mapstructure:"server"`
	GRPCServer  GRPCServerConfig   `mapstructure:"grpc_server"`
	IDGenerator IDGeneratorConfig  `mapstructure:"id_generator"`
	Database    postgres.Config    `mapstructure:"database"`
	Redis       commonRedis.Config `mapstructure:"redis"`
	OTel        commonOtel.Config  `mapstructure:"otel"`
	Metrics     MetricsConfig      `mapstructure:"metrics"`
	MinIO       MinIOConfig        `mapstructure:"minio"`
	AI          AIConfig           `mapstructure:"ai"`
}

// AIConfig AI 服务配置。
type AIConfig struct {
	Provider      string `mapstructure:"provider"`        // "anthropic" | "openai" | "ollama" | "qwen"
	BaseURL       string `mapstructure:"base_url"`         // LLM API 端点地址
	APIKey        string `mapstructure:"api_key"`          // API Key
	EnrichModel   string `mapstructure:"enrich_model"`     // 日常增值用的模型
	ReportModel   string `mapstructure:"report_model"`     // 周报生成用的模型（留空则用 enrich_model）
	MaxTokens     int    `mapstructure:"max_tokens"`       // 默认 1024
	WorkerCount   int    `mapstructure:"worker_count"`     // Worker 并发数，默认 4
	MinContentLen int    `mapstructure:"min_content_len"`  // 内容最小长度，默认 50
}

type GRPCServerConfig struct {
	Port string `mapstructure:"port"`
}

type IDGeneratorConfig struct {
	Addr string `mapstructure:"addr"`
}

type MetricsConfig struct {
	Port string `mapstructure:"port"`
}

// LoadConfig 从 Viper 加载配置
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)
	return &cfg
}
