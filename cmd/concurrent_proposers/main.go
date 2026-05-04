package main

import (
	"disk-paxos/internal/config"
	"disk-paxos/internal/logger"
	"disk-paxos/internal/processor"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

const retries = 10

func main() {
	cfg, log := setup()
	wg := new(sync.WaitGroup)
	for i := range cfg.ProcessorCount {

		id := i + 1
		value := fmt.Sprintf("value %d", id)
		proposer := processor.NewProcessor(id, cfg, log)

		wg.Go(func() {
			for range retries {
				_, err := proposer.Propose(value)
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

func setup() (cfg *config.Config, log *zap.Logger) {
	log, err := logger.NewConsoleLogger()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	cfg, err = config.LoadConfig()
	if err != nil {
		log.Fatal("", zap.Error(err))
	}
	return
}
