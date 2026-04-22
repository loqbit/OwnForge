// Package conf provides a unified Viper config loader and a few common config types.
//
// Design principles:
//   - Infrastructure config types (Redis / Postgres) are defined in their respective common packages
//   - Only common types without dedicated packages are kept here (OTel / Server / IDGenerator)
//   - Each service composes its Config struct by embedding shared types
package conf

import (
	"errors"
	"log"
	"reflect"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// ServerConfig is the common HTTP server configuration.
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// IDGeneratorConfig is the distributed ID generator configuration.
type IDGeneratorConfig struct {
	Addr string `mapstructure:"addr"`
}

// Load loads configuration into the target struct.
//
// It unifies repeated loading logic across services: godotenv -> viper reads YAML -> environment overrides -> Unmarshal.
// target must be a pointer (for example, &Config{}), and its fields can embed the shared types above,
// and can also contain service-specific types.
func Load(target any) {
	_ = godotenv.Load()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	// AutomaticEnv alone is not reliable enough for pure environment-variable setups with nested structs.
	// Here every mapstructure key in the struct is explicitly bound to ENV,
	// so Unmarshal works reliably even without a config.yaml file.
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

// BindEnvForStruct explicitly binds all mapstructure keys in the struct,
// ensuring that Viper can still unmarshal from environment variables when config.yaml is absent.
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

// bindStructEnv recursively scans struct fields and expands nested config into Viper keys.
// For example:
//
//	Server.Port -> server.port -> maps to environment variable SERVER_PORT
//	Redis.Addr  -> redis.addr  -> maps to environment variable REDIS_ADDR
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

		// If the field itself is still a struct, recurse into it,
		// and keep building paths such as server.port or redis.addr from its child fields.
		if fieldType.Kind() == reflect.Struct {
			bindStructEnv(fieldType, nextPrefix)
			continue
		}

		// Only non-struct leaf nodes actually call BindEnv.
		if nextPrefix != "" {
			_ = viper.BindEnv(nextPrefix)
		}
	}
}

// parseMapstructureTag parses the mapstructure tag.
// For example:
//
//	`mapstructure:"server"`      -> name=server
//	`mapstructure:",squash"`     -> squash=true
//	When the tag is absent, it falls back to the lowercase field name.
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
