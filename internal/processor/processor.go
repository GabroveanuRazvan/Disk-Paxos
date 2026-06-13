package processor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"

	"go.uber.org/zap"
)

var (
	ErrPhase1Preempted  = errors.New("aborted: higher ballot found in phase 1")
	ErrPhase2Preempted  = errors.New("aborted: higher ballot found in phase 2")
	ErrQuorumNotReached = errors.New("quorum not reached")
)

type Phase int

const (
	PhaseRecovery Phase = iota
	PhaseScout
	PhaseCommit
	PhaseDone
)

// Processor is our proposer in the algorithm
type Processor struct {
	ID      int
	cfg     *config.Config
	Block   *block.Block
	Phase   Phase
	logger  *zap.Logger
	clients []*disk.Client
}

func NewProcessor(id int, cfg *config.Config, logger *zap.Logger) *Processor {
	blk := new(block.Block)
	logg := logger.With(zap.Int("processor", id))

	return &Processor{
		ID:      id,
		cfg:     cfg,
		Block:   blk,
		Phase:   PhaseRecovery,
		logger:  logg,
		clients: disk.NewClients(cfg),
	}
}

func NewProcessorWithClients(id int, cfg *config.Config, logger *zap.Logger, clients ...*disk.Client) *Processor {
	blk := new(block.Block)
	logg := logger.With(zap.Int("processor", id))

	return &Processor{
		ID:      id,
		cfg:     cfg,
		Block:   blk,
		Phase:   PhaseRecovery,
		logger:  logg,
		clients: clients,
	}
}

// NextBallot computes the next mbal value higher than the last registered mbal.
func (p *Processor) NextBallot() int {
	n := p.cfg.ProcessorCount
	// mbal - last processor ID that commited this mbal + this processor ID
	// results in the nearest multiple of N; the multiplier being the round number
	b := p.Block.Mbal - (p.Block.Mbal % n) + p.ID
	// If the mbal is smaller advance to the next round => add N
	if b <= p.Block.Mbal {
		b += n
	}
	return b
}

func (p *Processor) Propose(ctx context.Context, value string) (string, error) {
	// ==========================================
	// PHASE 0: The Recovery
	// ==========================================

	if p.Phase == PhaseRecovery {
		if err := p.Recover(ctx); err != nil {
			return "", err
		}
	} else {
		p.startBallot()
	}

	// ==========================================
	// PHASE 1: The Scout
	// ==========================================

	p.logger.Debug("Proposing", zap.String("value", value), zap.Int("ballot", p.Block.Mbal))

	// Write our current block to all disks
	if err := p.WriteToDisks(ctx); err != nil {
		return "", err
	}

	// Read check the other processors' blocks from the disks
	allBlocks, err := p.ReadOtherBlocksFromDisks(ctx)
	if err != nil {
		return "", err
	}

	proposalValue := value
	highestBal := p.Block.Bal
	if p.Block.Bal > 0 {
		proposalValue = p.Block.Inp
	}

	for _, blk := range allBlocks {

		// Higher mbal than our block, we got preempted in phase 1
		if blk.Mbal > p.Block.Mbal {
			p.Block.Mbal = blk.Mbal
			p.Phase = PhaseScout
			return "", fmt.Errorf("%w: %d", ErrPhase1Preempted, blk.Mbal)
		}

		// Adoption check; update bal to highest bal and adopt the new proposal value
		if blk.Bal > highestBal {
			highestBal = blk.Bal
			proposalValue = blk.Inp
		}
	}

	// ==========================================
	// PHASE 2: The Commit
	// ==========================================

	p.Phase = PhaseCommit

	// Did not get preempted in phase 1, try to commit this block
	p.Block.Bal = p.Block.Mbal
	p.Block.Inp = proposalValue // either my value or the adopted value

	// Write the new block to all disks
	if err = p.WriteToDisks(ctx); err != nil {
		return "", err
	}

	// Final preemption check
	finalBlocks, err := p.ReadOtherBlocksFromDisks(ctx)
	if err != nil {
		return "", err
	}

	for _, blk := range finalBlocks {
		if blk.Mbal > p.Block.Mbal {
			p.Block.Mbal = blk.Mbal
			return "", fmt.Errorf("%w: %d", ErrPhase2Preempted, blk.Mbal)
		}
	}

	p.Phase = PhaseDone
	p.logger.Info("Consensus reached", zap.String("value", p.Block.Inp))
	return p.Block.Inp, nil
}

func (p *Processor) startBallot() {
	p.Phase = PhaseScout
	p.Block.Mbal = p.NextBallot()
}

func (p *Processor) Recover(ctx context.Context) error {
	p.logger.Debug("Recovering from disks")

	ownBlocks, err := p.ReadOwnBlocksFromDisks(ctx)
	if err != nil {
		return err
	}

	maxMbal := p.Block.Mbal
	recovered := *p.Block
	for _, blk := range ownBlocks {
		if blk.Mbal > maxMbal {
			maxMbal = blk.Mbal
		}
		if blk.Bal > recovered.Bal {
			recovered.Bal = blk.Bal
			recovered.Inp = blk.Inp
		}
	}

	p.Block.Mbal = maxMbal
	p.Block.Bal = recovered.Bal
	p.Block.Inp = recovered.Inp
	p.startBallot()

	return nil
}

// WriteToDisks writes the blocks in parallel to all disks.
// Fails if quorum is not reached.
func (p *Processor) WriteToDisks(ctx context.Context) error {
	p.logger.Debug("Writing to disks")
	wg := new(sync.WaitGroup)
	var successCount atomic.Int32

	for i := range p.cfg.DiskCount {
		wg.Go(func() {
			client := p.clients[i]
			if err := client.SetBlock(ctx, p.ID, p.Block); err != nil {
				p.logger.Warn("Failed to write block", zap.Error(err))
				return
			}
			successCount.Add(1)
		})

	}
	wg.Wait()

	if successCount.Load() < int32(p.cfg.DiskCount/2+1) {
		return ErrQuorumNotReached
	}

	return nil
}

// ReadFromDisks reads the blocks in parallel to all disks.
// Fails if quorum is not reached.
func (p *Processor) ReadFromDisks(ctx context.Context) ([]*block.Block, error) {
	ids := make([]int, 0, p.cfg.ProcessorCount)
	for i := range p.cfg.ProcessorCount {
		ids = append(ids, i+1)
	}

	return p.readBlocksFromDisks(ctx, ids...)
}

func (p *Processor) ReadOwnBlocksFromDisks(ctx context.Context) ([]*block.Block, error) {
	return p.readBlocksFromDisks(ctx, p.ID)
}

func (p *Processor) ReadOtherBlocksFromDisks(ctx context.Context) ([]*block.Block, error) {
	ids := make([]int, 0, p.cfg.ProcessorCount-1)
	for i := range p.cfg.ProcessorCount {
		id := i + 1
		if id != p.ID {
			ids = append(ids, id)
		}
	}

	return p.readBlocksFromDisks(ctx, ids...)
}

func (p *Processor) readBlocksFromDisks(ctx context.Context, ids ...int) ([]*block.Block, error) {
	p.logger.Debug("Reading from disks")
	wg := new(sync.WaitGroup)
	blocks := make([]*block.Block, 0)
	var blkMu sync.Mutex
	var successCount atomic.Int32

	for i := range p.cfg.DiskCount {

		wg.Go(func() {
			client := p.clients[i]

			currentBlk, err := client.ReadBlocks(ctx, ids...)
			if err != nil {
				p.logger.Warn("Read blocks error", zap.Error(err))
				return
			}

			successCount.Add(1)

			blkMu.Lock()
			defer blkMu.Unlock()
			blocks = append(blocks, currentBlk...)
		})

	}
	wg.Wait()

	if successCount.Load() < int32(p.cfg.DiskCount/2+1) {
		return []*block.Block{}, ErrQuorumNotReached
	}

	return blocks, nil
}

func (p *Processor) Close() {
	for _, client := range p.clients {
		_ = client.Close()
	}
}
