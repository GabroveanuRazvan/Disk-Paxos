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
	blk := &block.Block{
		Mbal: 0,
		Bal:  0,
	}

	return &Processor{
		ID:    id,
		cfg:   cfg,
		block: blk,
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

	allBlocks, err := p.ReadFromDisks(ctx)
	if err != nil {
		return err
	}

	highestBal := 0
	proposalValue := value

	for _, blk := range allBlocks {
		if blk.Mbal > p.block.Mbal {
			return fmt.Errorf("aborted in phase 1, higher mbal %d found", blk.Mbal)
		}

		// Adoption check
		if blk.Mbal > highestBal {
			highestBal = blk.Mbal
			proposalValue = blk.Inp
		}
	}

	// ==========================================
	// PHASE 2: The Commit
	// ==========================================

	p.block.Bal = p.block.Mbal
	p.block.Inp = proposalValue // either my value or the adopted value

	if err = p.WriteToDisks(ctx); err != nil {
		return err
	}

	finalBlocks, err := p.ReadFromDisks(ctx)
	if err != nil {
		return err
	}

	for _, blk := range finalBlocks {
		if blk.Mbal > p.block.Mbal {
			return fmt.Errorf("aborted in phase 2, higher mbal %d found", blk.Mbal)
		}
	}

	log.Printf("Consensus reached for value %s\n", proposalValue)
	return nil
}

func (p *Processor) WriteToDisks(ctx context.Context) error {
	wg := new(sync.WaitGroup)

	for i := range p.cfg.DiskCount {
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

func (p *Processor) ReadFromDisks(ctx context.Context) ([]*block.Block, error) {
	wg := new(sync.WaitGroup)

	blocks := make([]*block.Block, 0)
	var blkMu sync.Mutex

	for i := range p.cfg.DiskCount {
		addr := p.cfg.DiskAddresses[i]

		wg.Go(func() {
			client := disk.NewClient(p.cfg, addr)

			currentBlk, err := client.ReadAllBlocks(ctx)
			if err != nil {
				log.Println("Read blocks error:", err)
				return
			}

			blkMu.Lock()
			defer blkMu.Unlock()
			blocks = append(blocks, currentBlk...)
		})

	}
	wg.Wait()

	return blocks, nil
}
