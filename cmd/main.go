package main

import (
	"context"
	"fmt"

	"disk-paxos/internal/config"
	"disk-paxos/internal/logger"
	"disk-paxos/internal/processor"

	"go.uber.org/zap"
)

func main() {
	log, err := logger.NewConsoleLogger()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("", zap.Error(err))
	}

	p := processor.NewProcessor(1, cfg, log)
	if err := p.WriteToDisks(context.Background()); err != nil {
		log.Fatal("", zap.Error(err))
	}

	for i := range 3 {
		value := fmt.Sprintf("value %d", i)
		val, err := p.Propose(value)
		if err != nil {
			log.Fatal("", zap.Error(err))
		}

		log.Info("", zap.Any("value", val))

	}
}
