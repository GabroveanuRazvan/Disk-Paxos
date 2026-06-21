//go:build racebug

package processor

import (
	"context"
	"runtime"
	"testing"

	"disk-paxos/internal/block"

	"go.uber.org/zap"
)

type raceObservingStore struct{}

func (s *raceObservingStore) SetBlock(_ context.Context, _ int, blk *block.Block) error {
	for range 100_000 {
		_ = blk.Mbal
		_ = blk.Bal
		_ = blk.Inp
		runtime.Gosched()
	}
	return nil
}

func (s *raceObservingStore) ReadBlocks(context.Context, ...int) ([]*block.Block, error) {
	return nil, nil
}

func (s *raceObservingStore) ReadAllBlocks(context.Context) ([]*block.Block, error) {
	return nil, nil
}

func (s *raceObservingStore) DeleteAllBlocks(context.Context) error {
	return nil
}

func (s *raceObservingStore) Close() error {
	return nil
}

func TestInjectedRaceBugIsDetectedByRaceDetector(t *testing.T) {
	cfg := testConfig()
	cfg.InjectRaceBug = true

	stores := make([]Store, 0, cfg.DiskCount)
	for range cfg.DiskCount {
		stores = append(stores, &raceObservingStore{})
	}

	p := NewProcessorWithClients(1, cfg, zap.NewNop(), stores...)
	if err := p.WriteToDisks(context.Background()); err != nil {
		t.Fatalf("WriteToDisks returned error: %v", err)
	}
}
