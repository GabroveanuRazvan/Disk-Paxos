package processor

import (
	"context"
	"errors"
	"testing"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"
	"disk-paxos/internal/processor/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func testConfig() *config.Config {
	return &config.Config{
		ProcessorCount: 3,
		DiskCount:      3,
		DiskAddresses:  []string{"disk-1", "disk-2", "disk-3"},
		Retries:        3,
	}
}

func TestNextBallot(t *testing.T) {
	tests := []struct {
		name string
		id   int
		mbal int
		want int
	}{
		{name: "first ballot for processor one", id: 1, mbal: 0, want: 1},
		{name: "first ballot for processor two", id: 2, mbal: 0, want: 2},
		{name: "advance same processor to next round", id: 1, mbal: 1, want: 4},
		{name: "advance after higher ballot", id: 2, mbal: 8, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessorWithClients(tt.id, testConfig(), zap.NewNop())
			p.Block.Mbal = tt.mbal

			if got := p.NextBallot(); got != tt.want {
				t.Fatalf("NextBallot() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRecoverRestoresHighestAcceptedBlockAndStartsScout(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clients := mockStores(ctrl, 3)

	clients[0].EXPECT().
		ReadBlocks(ctx, 2).
		Return([]*block.Block{{Mbal: 5, Bal: 2, Inp: "old"}}, nil)
	clients[1].EXPECT().
		ReadBlocks(ctx, 2).
		Return([]*block.Block{{Mbal: 8, Bal: 7, Inp: "chosen"}}, nil)
	clients[2].EXPECT().
		ReadBlocks(ctx, 2).
		Return([]*block.Block{{Mbal: 6, Bal: 3, Inp: "ignored"}}, nil)

	p := NewProcessorWithClients(2, testConfig(), zap.NewNop(), mockStoresToStores(clients)...)

	if err := p.Recover(ctx); err != nil {
		t.Fatalf("Recover returned error: %v", err)
	}

	if p.Phase != PhaseScout {
		t.Fatalf("phase = %v, want %v", p.Phase, PhaseScout)
	}
	if p.Block.Mbal != 11 {
		t.Fatalf("recovered mbal = %d, want 11", p.Block.Mbal)
	}
	if p.Block.Bal != 7 {
		t.Fatalf("recovered bal = %d, want 7", p.Block.Bal)
	}
	if p.Block.Inp != "chosen" {
		t.Fatalf("recovered input = %q, want %q", p.Block.Inp, "chosen")
	}
}

func TestWriteToDisksRequiresQuorum(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clients := mockStores(ctrl, 3)
	writeErr := errors.New("write failed")

	clients[0].EXPECT().SetBlock(ctx, 1, gomock.Any()).Return(nil)
	clients[1].EXPECT().SetBlock(ctx, 1, gomock.Any()).Return(writeErr)
	clients[2].EXPECT().SetBlock(ctx, 1, gomock.Any()).Return(writeErr)

	p := NewProcessorWithClients(1, testConfig(), zap.NewNop(), mockStoresToStores(clients)...)

	err := p.WriteToDisks(ctx)
	if !errors.Is(err, ErrQuorumNotReached) {
		t.Fatalf("WriteToDisks error = %v, want %v", err, ErrQuorumNotReached)
	}
}

func TestReadOtherBlocksFromDisksRequiresQuorum(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clients := mockStores(ctrl, 3)
	readErr := errors.New("read failed")

	clients[0].EXPECT().ReadBlocks(ctx, 2, 3).Return([]*block.Block{}, nil)
	clients[1].EXPECT().ReadBlocks(ctx, 2, 3).Return(nil, readErr)
	clients[2].EXPECT().ReadBlocks(ctx, 2, 3).Return(nil, readErr)

	p := NewProcessorWithClients(1, testConfig(), zap.NewNop(), mockStoresToStores(clients)...)

	blocks, err := p.ReadOtherBlocksFromDisks(ctx)
	if !errors.Is(err, ErrQuorumNotReached) {
		t.Fatalf("ReadOtherBlocksFromDisks error = %v, want %v", err, ErrQuorumNotReached)
	}
	if len(blocks) != 0 {
		t.Fatalf("ReadOtherBlocksFromDisks returned %d blocks, want 0", len(blocks))
	}
}

func TestProposeAdoptsHighestAcceptedValue(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	clients := mockStores(ctrl, 3)
	cfg := testConfig()
	adopted := []*block.Block{
		{Mbal: 5, Bal: 5, Inp: "adopted"},
		{Mbal: 4, Bal: 4, Inp: "older"},
	}

	for _, client := range clients {
		gomock.InOrder(
			client.EXPECT().
				SetBlock(ctx, 1, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ int, blk *block.Block) error {
					if blk.Bal != 0 {
						t.Fatalf("phase 1 wrote bal %d, want 0", blk.Bal)
					}
					if blk.Inp != "" {
						t.Fatalf("phase 1 wrote inp %q, want empty string", blk.Inp)
					}
					return nil
				}),
			client.EXPECT().
				ReadBlocks(ctx, 2, 3).
				Return(adopted, nil),
			client.EXPECT().
				SetBlock(ctx, 1, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ int, blk *block.Block) error {
					if blk.Bal != 7 {
						t.Fatalf("phase 2 wrote bal %d, want 7", blk.Bal)
					}
					if blk.Inp != "adopted" {
						t.Fatalf("phase 2 wrote inp %q, want %q", blk.Inp, "adopted")
					}
					return nil
				}),
			client.EXPECT().
				ReadBlocks(ctx, 2, 3).
				Return([]*block.Block{}, nil),
		)
	}

	p := NewProcessorWithClients(1, cfg, zap.NewNop(), mockStoresToStores(clients)...)
	p.Phase = PhaseScout
	p.Block.Mbal = 6

	decided, err := p.Propose(ctx, "mine")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if decided != "adopted" {
		t.Fatalf("Propose decided %q, want %q", decided, "adopted")
	}
	if p.Phase != PhaseDone {
		t.Fatalf("phase = %v, want %v", p.Phase, PhaseDone)
	}
}

func mockStores(ctrl *gomock.Controller, count int) []*mocks.MockStore {
	clients := make([]*mocks.MockStore, 0, count)
	for range count {
		clients = append(clients, mocks.NewMockStore(ctrl))
	}
	return clients
}

func mockStoresToStores(clients []*mocks.MockStore) []Store {
	stores := make([]Store, 0, len(clients))
	for _, client := range clients {
		stores = append(stores, client)
	}
	return stores
}
