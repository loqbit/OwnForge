package config

import (
	"github.com/loqbit/ownforge/pkg/conf"
	commonOtel "github.com/loqbit/ownforge/pkg/otel"
	"github.com/loqbit/ownforge/pkg/postgres"
	commonRedis "github.com/loqbit/ownforge/pkg/redis"
)

// MinIOConfig defines MinIO connection settings.
type MinIOConfig struct {
	Endpoint       string   `mapstructure:"endpoint"`        // internal endpoint (Docker: global-minio:9000)
	PublicEndpoint string   `mapstructure:"public_endpoint"` // browser-accessible endpoint (localhost:9000)
	AccessKey      string   `mapstructure:"access_key"`
	SecretKey      string   `mapstructure:"secret_key"`
	Bucket         string   `mapstructure:"bucket"`
	UseSSL         bool     `mapstructure:"use_ssl"`
	PresignExpiry  int      `mapstructure:"presign_expiry"`
	MaxUploadSize  int64    `mapstructure:"max_upload_size"`
	AllowedMIMEs   []string `mapstructure:"allowed_mime_types"`
}

// Config holds global configuration.
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

// AIConfig defines AI service settings.
type AIConfig struct {
	Provider      string `mapstructure:"provider"`        // "anthropic" | "openai" | "ollama" | "qwen"
	BaseURL       string `mapstructure:"base_url"`        // LLM API endpoint
	APIKey        string `mapstructure:"api_key"`         // API Key
	EnrichModel   string `mapstructure:"enrich_model"`    // model used for routine enrichment
	ReportModel   string `mapstructure:"report_model"`    // model used for weekly report generation (falls back to enrich_model when empty)
	MaxTokens     int    `mapstructure:"max_tokens"`      // defaults to 1024
	WorkerCount   int    `mapstructure:"worker_count"`    // worker concurrency, default 4
	MinContentLen int    `mapstructure:"min_content_len"` // minimum content length, default 50
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

// LoadConfig loads configuration from Viper.
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)
	return &cfg
}
