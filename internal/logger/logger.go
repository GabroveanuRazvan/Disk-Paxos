package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewConsoleLogger() (*zap.Logger, error) {
	// 1. Start with the default development configuration.
	// This automatically sets the output to standard console format (not JSON)
	// and sets the default logging level to Debug.
	config := zap.NewDevelopmentConfig()

	// 2. Override the level encoder to add colors.
	// This will make "DEBUG" magenta, "INFO" blue, "WARN" yellow, and "ERROR" red.
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Optional: You can also tweak other encoder settings here.
	// For example, to simplify the timestamp format:
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")

	// 3. Build and return the logger
	return config.Build()
}
