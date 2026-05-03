package disk

import (
	"context"
	"fmt"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"

	"github.com/redis/go-redis/v9"
)

const blockKeyFormat = "block-%d"

type Client struct {
	cfg    *config.Config
	client *redis.Client
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

func (c *Client) SetBlock(ctx context.Context, id int, block *block.Block) error {
	key := fmt.Sprintf(blockKeyFormat, id)
	return c.client.Set(ctx, key, block.JSON(), 0).Err()
}

func (c *Client) ReadBlocks(ctx context.Context, ids ...int) ([]*block.Block, error) {
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

func (c *Client) ReadAllBlocks(ctx context.Context) ([]*block.Block, error) {
	ids := make([]int, 0, c.cfg.ProcessorCount)
	for i := range c.cfg.ProcessorCount {
		ids = append(ids, i+1)
	}

	return c.ReadBlocks(ctx, ids...)
}

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
