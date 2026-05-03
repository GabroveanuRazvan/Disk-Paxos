package processor

import (
	"context"
	"disk-paxos/internal/block"
	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"
	"fmt"
	"log"
	"sync"
)

const blockKeyFormat = "block-%d"

type Processor struct {
	ID    int
	cfg   *config.Config
	block *block.Block
}

func NewProcessor(id int, cfg *config.Config) *Processor {
	block := &block.Block{
		Mbal: 0,
		Bal:  0,
	}

	return &Processor{
		ID:    id,
		cfg:   cfg,
		block: block,
	}
}

func (p *Processor) BlockID() string {
	return fmt.Sprintf(blockKeyFormat, p.ID)
}

func (p *Processor) NextBallot() int {
	return p.block.Mbal + p.ID
}

func (p *Processor) Propose(value string) error {

	// ==========================================
	// PHASE 1: The Scout
	// ==========================================

	p.block.Inp = value
	p.block.Mbal = p.NextBallot()

	ctx := context.Background()

	if err := p.WriteToDisks(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Processor) WriteToDisks(ctx context.Context) error {
	wg := new(sync.WaitGroup)

	for i := 0; i < p.cfg.DiskCount; i++ {
		addr := p.cfg.DiskAddresses[i]

		wg.Go(func() {
			client := disk.NewClient(p.cfg, addr)
			defer client.Close()

			if err := client.SetBlock(ctx, p.ID, p.block); err != nil {
				log.Println("Set error", err)
				return
			}

		})

	}
	wg.Wait()

	return nil
}

func (p *Processor) ReadFromDisks(ctx context.Context) error {
	wg := new(sync.WaitGroup)

	blockKeys := make([]string, 0, p.cfg.ProcessorCount)

	for i := 0; i < p.cfg.ProcessorCount; i++ {
		blockKeys = append(blockKeys, fmt.Sprintf(blockKeyFormat, i))
	}

	for i := 0; i < p.cfg.DiskCount; i++ {
		addr := p.cfg.DiskAddresses[i]
		// TODO
	}

	wg.Wait()
	return nil
}
