package disk

import (
	"context"
	"disk-paxos/internal/config"
	"disk-paxos/internal/processor"
	"fmt"

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

func (c *Client) SetBlock(ctx context.Context, id int, block processor.Block) error {
	key := fmt.Sprintf(blockKeyFormat, id)
	return c.client.Set(ctx, key, block.JSON(), 0).Err()
}

func (c *Client) ReadBlocks(ctx context.Context, ids ...int) ([]interface{}, error) {
	keys := make([]string, 0, len(ids))
	for i := 0; i < len(ids); i++ {
		keys = append(keys, fmt.Sprintf(blockKeyFormat, ids[i]))
	}

	return c.client.MGet(ctx, keys...).Result()
}

func (c *Client) Close() error {
	return c.client.Close()
}
