// Package conf 提供统一的 Viper 配置加载函数和少量通用配置类型。
//
// 设计原则：
//   - 基础设施配置类型（Redis / Postgres）定义在各自的 common 包中
//   - 只保留无对应包的通用类型（OTel / Server / IDGenerator）
//   - 各服务通过嵌入公共类型来组合自己的 Config 结构体
package conf

import (
	"errors"
	"log"
	"reflect"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// ServerConfig 通用 HTTP 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// IDGeneratorConfig 分布式 ID 生成器配置
type IDGeneratorConfig struct {
	Addr string `mapstructure:"addr"`
}

// Load 加载配置到目标结构体。
//
// 统一了各服务重复的加载逻辑：godotenv → viper 读 YAML → 环境变量覆盖 → Unmarshal。
// target 必须是指针类型（例如 &Config{}），其字段可以嵌入上面的公共类型，
// 也可以包含服务专有的类型。
func Load(target any) {
	_ = godotenv.Load()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	// 仅开启 AutomaticEnv 对“纯环境变量 + 嵌套结构体”的场景不够稳。
	// 这里把结构体里的每个 mapstructure key 都显式绑定到 ENV，
	// 这样即使没有 config.yaml，也能可靠地完成 Unmarshal。
	BindEnvForStruct(target)

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			log.Printf("Warning: Failed to read config file: %v", err)
		}
	}

	if err := viper.Unmarshal(target); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
}

// BindEnvForStruct 显式绑定结构体里的所有 mapstructure key，
// 确保在没有 config.yaml 时，Viper 仍然能通过环境变量完成 Unmarshal。
func BindEnvForStruct(target any) {
	if target == nil {
		return
	}

	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	bindStructEnv(t, "")
}

// bindStructEnv 递归扫描结构体字段，把嵌套配置展开成 Viper key。
// 例如：
//
//	Server.Port -> server.port -> 对应环境变量 SERVER_PORT
//	Redis.Addr  -> redis.addr  -> 对应环境变量 REDIS_ADDR
func bindStructEnv(t reflect.Type, prefix string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("mapstructure")
		if tag == "-" {
			continue
		}

		name, squash := parseMapstructureTag(tag, field.Name)
		nextPrefix := prefix
		if !squash && name != "" {
			if prefix == "" {
				nextPrefix = name
			} else {
				nextPrefix = prefix + "." + name
			}
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		// 如果字段本身还是结构体，就继续向下递归，
		// 把它的子字段继续拼成 server.port / redis.addr 这种路径。
		if fieldType.Kind() == reflect.Struct {
			bindStructEnv(fieldType, nextPrefix)
			continue
		}

		// 非结构体叶子节点才真正执行 BindEnv。
		if nextPrefix != "" {
			_ = viper.BindEnv(nextPrefix)
		}
	}
}

// parseMapstructureTag 解析 mapstructure 标签。
// 例如：
//
//	`mapstructure:"server"`      -> name=server
//	`mapstructure:",squash"`     -> squash=true
//	无标签时回退为字段名小写。
func parseMapstructureTag(tag string, fallback string) (name string, squash bool) {
	if tag == "" {
		return strings.ToLower(fallback), false
	}

	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, part := range parts[1:] {
		if part == "squash" {
			squash = true
		}
	}

	if name == "" {
		name = strings.ToLower(fallback)
	}

	return name, squash
}
