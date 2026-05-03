package main

import (
	"disk-paxos/internal/logger"
	"log"

	"go.uber.org/zap"
)

func main() {
	// Initialize the logger
	logger, err := logger.NewConsoleLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Best practice: ensure buffered logs are flushed before the app exits
	defer logger.Sync()

	// Test the colorful output
	logger.Debug("This is a debug message", zap.String("context", "setup"))
	logger.Info("App is starting up normally", zap.Int("port", 8080))
	logger.Warn("Disk space is getting a bit low", zap.Int("free_mb", 512))
	logger.Error("Failed to connect to the database", zap.Error(err))
}
