package processor

import (
	"context"

	"disk-paxos/internal/block"
)

//go:generate go run go.uber.org/mock/mockgen -source=store.go -destination=../../mocks/mock_store.go -package=mocks

type Store interface {
	SetBlock(ctx context.Context, id int, block *block.Block) error
	ReadBlocks(ctx context.Context, ids ...int) ([]*block.Block, error)
	ReadAllBlocks(ctx context.Context) ([]*block.Block, error)
	DeleteAllBlocks(ctx context.Context) error
	Close() error
}
