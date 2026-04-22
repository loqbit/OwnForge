# common/conf — 统一配置加载

提供微服务间共享的通用配置类型和统一的 Viper 配置加载函数。

## 公共类型

```go
type DatabaseConfig struct { Driver, Source string; AutoMigrate bool }
type RedisConfig    struct { Addr, Password string; DB int }
type OTelConfig     struct { ServiceName, JaegerEndpoint string }
type ServerConfig   struct { Port string }
type IDGeneratorConfig struct { Addr string }
```

## 用法

各服务通过**嵌入**公共类型组合自己的 Config：

```go
// go-note/internal/platform/config/config.go
type Config struct {
    Database    conf.DatabaseConfig    `mapstructure:"database"`
    Redis       conf.RedisConfig       `mapstructure:"redis"`
    OTel        conf.OTelConfig        `mapstructure:"otel"`
    Server      conf.ServerConfig      `mapstructure:"server"`
    IDGenerator conf.IDGeneratorConfig `mapstructure:"id_generator"`
    // 服务专有配置
    MyFeature   MyFeatureConfig        `mapstructure:"my_feature"`
}

func LoadConfig() *Config {
    var cfg Config
    conf.Load(&cfg)  // godotenv → viper → env override → unmarshal
    return &cfg
}
```

## 设计原则

- ✅ 只抽取**完全重复**的配置类型
- ✅ 服务专有配置（JWT / Kafka / Chat）仍由各服务自行定义
- ✅ 支持 YAML 配置文件 + 环境变量覆盖
