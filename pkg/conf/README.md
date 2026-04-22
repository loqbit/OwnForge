# common/conf — Unified Config Loading

Provides shared config types across microservices and a unified Viper config loader.

## Shared Types

```go
type DatabaseConfig struct { Driver, Source string; AutoMigrate bool }
type RedisConfig    struct { Addr, Password string; DB int }
type OTelConfig     struct { ServiceName, JaegerEndpoint string }
type ServerConfig   struct { Port string }
type IDGeneratorConfig struct { Addr string }
```

## Usage

Each service composes its Config by **embedding** shared types:

```go
// go-note/internal/platform/config/config.go
type Config struct {
    Database    conf.DatabaseConfig    `mapstructure:"database"`
    Redis       conf.RedisConfig       `mapstructure:"redis"`
    OTel        conf.OTelConfig        `mapstructure:"otel"`
    Server      conf.ServerConfig      `mapstructure:"server"`
    IDGenerator conf.IDGeneratorConfig `mapstructure:"id_generator"`
    // service-specific configuration
    MyFeature   MyFeatureConfig        `mapstructure:"my_feature"`
}

func LoadConfig() *Config {
    var cfg Config
    conf.Load(&cfg)  // godotenv → viper → env override → unmarshal
    return &cfg
}
```

## Design Principles

- ✅ Only extract config types that are **fully duplicated**
- ✅ service-specific configuration（JWT / Kafka / Chat）is still defined by each service
- ✅ Supports YAML config files plus environment-variable overrides
