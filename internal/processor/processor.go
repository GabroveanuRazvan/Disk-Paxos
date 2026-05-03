package processor

import (
	"disk-paxos/internal/config"
	"fmt"
)

type Processor struct {
	ID    int
	cfg   config.Config
	block *Block
}

func NewProcessor(id int, cfg config.Config) *Processor {
	block := &Block{
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
	return fmt.Sprintf("block-%d", p.ID)
}

func (p *Processor) NextBallot() int {
	return p.block.Mbal + p.ID
}

func (p *Processor) Propose(value string) {

	p.block.Inp = value
	p.block.Mbal = p.NextBallot()

}

func (p *Processor) WriteToDisks() error {
	return nil
}
