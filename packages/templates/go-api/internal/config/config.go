package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppName        string
	AppEnv         string
	Port           string
	AllowedOrigins []string
}

func Load() Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("app.name", "go-api")
	v.SetDefault("app.env", "development")
	v.SetDefault("port", "3000")
	v.SetDefault("cors.allowed_origins", []string{
		"http://localhost:3000",
		"http://localhost:5173",
	})

	_ = v.ReadInConfig()

	return Config{
		AppName:        v.GetString("app.name"),
		AppEnv:         v.GetString("app.env"),
		Port:           v.GetString("port"),
		AllowedOrigins: getStringSlice(v, "cors.allowed_origins"),
	}
}

func getStringSlice(v *viper.Viper, key string) []string {
	value := v.Get(key)
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				result = append(result, strings.TrimSpace(value))
			}
		}
		return result
	}

	raw := strings.TrimSpace(v.GetString(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
