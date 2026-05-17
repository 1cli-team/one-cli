package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(env string) *zap.Logger {
	if env == "production" {
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "time"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		return zap.Must(cfg.Build(zap.AddCaller()))
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return zap.Must(cfg.Build(zap.AddCaller()))
}
