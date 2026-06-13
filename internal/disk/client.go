package disk

import (
	"context"
	"fmt"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const blockKeyFormat = "block-%d"

// Client is a wrapper over a redis client, used as our disk needed for the algorithm.
type Client struct {
	cfg    *config.Config
	client redis.UniversalClient
}

func NewClient(cfg *config.Config, addr string) *Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &Client{
		cfg:    cfg,
		client: redisClient,
	}
}

// NewClients creates all clients for the current config
func NewClients(cfg *config.Config) []*Client {
	clients := make([]*Client, 0, cfg.DiskCount)
	for _, addr := range cfg.DiskAddresses {
		client := NewClient(cfg, addr)
		clients = append(clients, client)
	}
	return clients
}

// SetBlock sets the block as a JSON string for the current id.
func (c *Client) SetBlock(ctx context.Context, id int, block *block.Block) error {
	key := fmt.Sprintf(blockKeyFormat, id)
	return c.client.Set(ctx, key, block.JSON(), 0).Err()
}

// ReadBlocks reads all blocks for the provided ids.
func (c *Client) ReadBlocks(ctx context.Context, ids ...int) ([]*block.Block, error) {
	if len(ids) == 0 {
		return []*block.Block{}, nil
	}

	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, fmt.Sprintf(blockKeyFormat, id))
	}

	data, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	blocks := make([]*block.Block, 0, len(data))

	for _, res := range data {
		if res == nil {
			continue
		}

		blk := new(block.Block)
		strBlk := res.(string)
		blk.FromJSON(strBlk)

		blocks = append(blocks, blk)
	}

	return blocks, nil
}

// ReadAllBlocks reads all blocks for all ids.
func (c *Client) ReadAllBlocks(ctx context.Context) ([]*block.Block, error) {
	ids := make([]int, 0, c.cfg.ProcessorCount)
	for i := range c.cfg.ProcessorCount {
		ids = append(ids, i+1)
	}

	return c.ReadBlocks(ctx, ids...)
}

// DeleteAllBlocks deletes all blocks for all ids.
func (c *Client) DeleteAllBlocks(ctx context.Context) error {
	keys := make([]string, 0, c.cfg.ProcessorCount)
	for i := range c.cfg.ProcessorCount {
		keys = append(keys, fmt.Sprintf(blockKeyFormat, i+1))
	}

	return c.client.Del(ctx, keys...).Err()
}

func (c *Client) Close() error {
	return c.client.Close()
}

// PurgeAllBlocks deletes all blocks for all disks.
func PurgeAllBlocks(ctx context.Context, cfg *config.Config, logger *zap.Logger) {
	clients := NewClients(cfg)

	for i, client := range clients {
		if err := client.DeleteAllBlocks(ctx); err != nil {
			logger.Warn("Failed to delete all disk blocks", zap.Int("index", i))
		}
	}
}
