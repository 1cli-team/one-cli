// Package config 加载服务配置。
//
// 配置加载策略（One CLI 推荐）：
//  1. configs/config.yaml 提供 schema 与本地开发默认值（committed）。
//  2. 环境变量按 SetEnvKeyReplacer 规则覆盖文件值：
//     viper.Get("database.url")  ←  env var DATABASE_URL
//     viper.Get("jwt.secret")    ←  env var JWT_SECRET
//     viper.Get("app.env")       ←  env var APP_ENV
//     （嵌套路径 a.b.c ↔ env var A_B_C）
//  3. 通过 `one env set DATABASE_URL=...` 设置的 secret 会在
//     `one run` / `one dev` 启动时被自动注入为环境变量，
//     于是服务无须手写 config 文件就能拿到生产值 —— 嵌套覆盖
//     全部由 viper AutomaticEnv 在运行时完成。
//
// 因此**不要**让任何工具去生成 configs/config.yaml；它就是默认值
// 来源 + schema 文档。生产敏感值改用 `one env set <KEY>=...`。
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppName        string
	AppEnv         string
	Port           string
	DatabaseURL    string
	JWTSecret      string
	JWTExpiresIn   time.Duration
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
	// database.url 默认留空 — 启动时回退到内存 SQLite（dev fallback）。
	// 需要真实 Postgres 时 `one env set DATABASE_URL=...` 或在
	// configs/config.yaml 里设。
	v.SetDefault("database.url", "")
	v.SetDefault("jwt.secret", "change-me")
	v.SetDefault("jwt.expires_in", "720h")
	v.SetDefault("cors.allowed_origins", []string{
		"http://localhost:3000",
		"http://localhost:5173",
	})

	_ = v.ReadInConfig()

	return Config{
		AppName:        v.GetString("app.name"),
		AppEnv:         v.GetString("app.env"),
		Port:           v.GetString("port"),
		DatabaseURL:    v.GetString("database.url"),
		JWTSecret:      v.GetString("jwt.secret"),
		JWTExpiresIn:   getDuration(v, "jwt.expires_in", 30*24*time.Hour),
		AllowedOrigins: getStringSlice(v, "cors.allowed_origins"),
	}
}

func getDuration(v *viper.Viper, key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(v.GetString(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
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
