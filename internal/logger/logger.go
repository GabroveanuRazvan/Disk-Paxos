package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewConsoleLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
	return config.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}
