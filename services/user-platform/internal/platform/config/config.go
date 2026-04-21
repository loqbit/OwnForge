package config

import (
	"github.com/luckysxx/common/conf"
	commonOtel "github.com/luckysxx/common/otel"
	"github.com/luckysxx/common/postgres"
	commonRedis "github.com/luckysxx/common/redis"
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

// === 以下为服务专有配置，不提取到 common ===

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

// SMSAuthConfig 定义手机号认证使用的阿里云短信验证码配置。
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

// LoadConfig 从 Viper 加载配置（底层由 common/conf.Load 统一处理）
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)
	return &cfg
}
