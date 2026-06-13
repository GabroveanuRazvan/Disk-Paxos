package main

import (
	"context"

	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"
	"disk-paxos/internal/logger"

	"go.uber.org/zap"
)

func main() {
	log, err := logger.NewConsoleLogger()
	if err != nil {
		panic(err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("", zap.Error(err))
	}

	ctx := context.Background()
	disk.PurgeAllBlocks(ctx, cfg, log)

	log.Info("Successfully cleared all blocks")
}
