package main

import (
	"context"
	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"
	"disk-paxos/internal/logger"
	"disk-paxos/internal/processor"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

func main() {
	cfg, log, clients := setup()
	ctx := context.Background()
	defer log.Sync()

	disk.PurgeAllBlocks(ctx, cfg, log)

	wg := new(sync.WaitGroup)
	for i := range cfg.ProcessorCount {

		id := i + 1
		value := fmt.Sprintf("value %d", id)
		proposer := processor.NewProcessorWithClients(id, cfg, log, clients...)

		wg.Go(func() {
			for range cfg.Retries {
				_, err := proposer.Propose(ctx, value)
				if err != nil {
					log.Warn("Proposer", zap.Int("id", id), zap.Error(err))
					continue
				}
				return
			}
		})

	}

	wg.Wait()

}

func setup() (cfg *config.Config, log *zap.Logger, clients []*disk.Client) {
	log, err := logger.NewConsoleLogger()
	if err != nil {
		panic(err)
	}

	cfg, err = config.LoadConfig()
	if err != nil {
		log.Fatal("", zap.Error(err))
	}
	clients = disk.NewClients(cfg)
	return
}
