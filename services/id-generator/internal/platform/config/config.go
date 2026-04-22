package config

import (
	"errors"
	"log"
	"strings"

	"github.com/joho/godotenv"
	commonconf "github.com/ownforge/ownforge/pkg/conf"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv    string          `mapstructure:"app_env"`
	Server    ServerConfig    `mapstructure:"server"`
	Snowflake SnowflakeConfig `mapstructure:"snowflake"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type SnowflakeConfig struct {
	NodeID int64 `mapstructure:"node_id"`
}

// LoadConfig loads configuration from Viper and supports overrides from .env.
func LoadConfig() *Config {
	_ = godotenv.Load()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/app/configs")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	commonconf.BindEnvForStruct(&Config{})

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			log.Printf("warning: failed to read config file: %v", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	return &cfg
}
