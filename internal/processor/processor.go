package processor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"

	"go.uber.org/zap"
)

var (
	ErrPhase1Preempted = errors.New("aborted: higher ballot found in phase 1")
	ErrPhase2Preempted = errors.New("aborted: higher ballot found in phase 2")
)

type Processor struct {
	ID      int
	cfg     *config.Config
	block   *block.Block
	logger  *zap.Logger
	clients []*disk.Client
}

func NewProcessor(id int, cfg *config.Config, logger *zap.Logger) *Processor {
	blk := &block.Block{
		Mbal: 0,
		Bal:  0,
	}
	logg := logger.With(zap.Int("processor", id))

	clients := make([]*disk.Client, 0, cfg.DiskCount)
	for _, addr := range cfg.DiskAddresses {
		client := disk.NewClient(cfg, addr)
		clients = append(clients, client)
	}

	return &Processor{
		ID:      id,
		cfg:     cfg,
		block:   blk,
		logger:  logg,
		clients: clients,
	}
}

func (p *Processor) NextBallot() int {
	return p.block.Mbal + p.ID
}

func (p *Processor) Propose(value string) (string, error) {
	// ==========================================
	// PHASE 1: The Scout
	// ==========================================

	p.block.Inp = value
	p.block.Mbal = p.NextBallot()
	ctx := context.Background()

	p.logger.Debug("Proposing", zap.String("value", value), zap.Int("ballot", p.block.Mbal))

	if err := p.WriteToDisks(ctx); err != nil {
		return "", err
	}

	allBlocks, err := p.ReadFromDisks(ctx)
	if err != nil {
		return "", err
	}

	highestBal := 0
	proposalValue := value

	for _, blk := range allBlocks {
		if blk.Mbal > p.block.Mbal {
			return "", fmt.Errorf("%w: %d", ErrPhase1Preempted, blk.Mbal)
		}

		// Adoption check
		if blk.Bal > highestBal {
			highestBal = blk.Bal
			proposalValue = blk.Inp
		}
	}

	// ==========================================
	// PHASE 2: The Commit
	// ==========================================

	p.block.Bal = p.block.Mbal
	p.block.Inp = proposalValue // either my value or the adopted value

	if err = p.WriteToDisks(ctx); err != nil {
		return "", err
	}

	finalBlocks, err := p.ReadFromDisks(ctx)
	if err != nil {
		return "", err
	}

	for _, blk := range finalBlocks {
		if blk.Mbal > p.block.Mbal {
			return "", fmt.Errorf("%w: %d", ErrPhase2Preempted, blk.Mbal)
		}
	}

	p.logger.Info("Consensus reached", zap.String("value", p.block.Inp))
	return p.block.Inp, nil
}

func (p *Processor) WriteToDisks(ctx context.Context) error {
	p.logger.Debug("Writing to disks")
	wg := new(sync.WaitGroup)

	for i := range p.cfg.DiskCount {

		wg.Go(func() {
			client := p.clients[i]

			if err := client.SetBlock(ctx, p.ID, p.block); err != nil {
				p.logger.Warn("Failed to write block", zap.Error(err))
				return
			}
		})

	}
	wg.Wait()

	return nil
}

func (p *Processor) ReadFromDisks(ctx context.Context) ([]*block.Block, error) {
	p.logger.Debug("Reading from disks")
	wg := new(sync.WaitGroup)

	blocks := make([]*block.Block, 0)
	var blkMu sync.Mutex

	for i := range p.cfg.DiskCount {

		wg.Go(func() {
			client := p.clients[i]

			currentBlk, err := client.ReadAllBlocks(ctx)
			if err != nil {
				p.logger.Warn("Read all blocks error", zap.Error(err))
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

func (p *Processor) Close() {
	for _, client := range p.clients {
		_ = client.Close()
	}
}
