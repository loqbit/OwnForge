package config

import (
	"github.com/loqbit/ownforge/pkg/conf"
	commonOtel "github.com/loqbit/ownforge/pkg/otel"
	"github.com/loqbit/ownforge/pkg/postgres"
	commonRedis "github.com/loqbit/ownforge/pkg/redis"
)

type Config struct {
	AppEnv      string                 `mapstructure:"app_env"`
	Server      conf.ServerConfig      `mapstructure:"server"`
	GRPCServer  GRPCServerConfig       `mapstructure:"grpc_server"`
	Database    postgres.Config        `mapstructure:"database"`
	Redis       commonRedis.Config     `mapstructure:"redis"`
	JWT         JWTConfig              `mapstructure:"jwt"`
	Kafka       KafkaConfig            `mapstructure:"kafka"`
	SMSAuth     SMSAuthConfig          `mapstructure:"sms_auth"`
	IDGenerator conf.IDGeneratorConfig `mapstructure:"id_generator"`
	OTel        commonOtel.Config      `mapstructure:"otel"`
	Metrics     MetricsConfig          `mapstructure:"metrics"`
}

// === Service-specific configuration below; do not extract into common. ===

type MetricsConfig struct {
	Port string `mapstructure:"port"`
}

type KafkaConfig struct {
	Brokers             string `mapstructure:"brokers"`
	TopicUserRegistered string `mapstructure:"topic_user_registered"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

type GRPCServerConfig struct {
	Port string `mapstructure:"port"`
}

// SMSAuthConfig defines the Aliyun SMS verification settings used for phone authentication.
type SMSAuthConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	AccessKeyID       string `mapstructure:"access_key_id"`
	AccessKeySecret   string `mapstructure:"access_key_secret"`
	Region            string `mapstructure:"region"`
	DebugMode         bool   `mapstructure:"debug_mode"`
	SignName          string `mapstructure:"sign_name"`
	TemplateCode      string `mapstructure:"template_code"`
	SchemeName        string `mapstructure:"scheme_name"`
	CountryCode       string `mapstructure:"country_code"`
	TemplateParamJSON string `mapstructure:"template_param_json"`
	CodeLength        int64  `mapstructure:"code_length"`
	IntervalSeconds   int64  `mapstructure:"interval_seconds"`
	ValidTimeSeconds  int64  `mapstructure:"valid_time_seconds"`
	CodeType          int64  `mapstructure:"code_type"`
	DuplicatePolicy   int64  `mapstructure:"duplicate_policy"`
	AutoRetry         int64  `mapstructure:"auto_retry"`
}

// LoadConfig loads configuration from Viper, with common/conf.Load handling the shared low-level logic.
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)
	return &cfg
}
