//go:build integration

package disk

import (
	"context"
	"testing"
	"time"

	"disk-paxos/internal/block"
	"disk-paxos/internal/config"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestClientRedisIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("redis mapped port: %v", err)
	}

	cfg := &config.Config{
		ProcessorCount: 3,
		DiskCount:      1,
		DiskAddresses:  []string{host + ":" + port.Port()},
		Retries:        1,
	}
	client := NewClient(cfg, cfg.DiskAddresses[0])
	t.Cleanup(func() {
		_ = client.Close()
	})

	want := &block.Block{
		Mbal: 4,
		Bal:  4,
		Inp:  "accepted",
	}
	if err := client.SetBlock(ctx, 2, want); err != nil {
		t.Fatalf("SetBlock returned error: %v", err)
	}

	blocks, err := client.ReadBlocks(ctx, 2)
	if err != nil {
		t.Fatalf("ReadBlocks returned error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("ReadBlocks returned %d blocks, want 1", len(blocks))
	}
	if *blocks[0] != *want {
		t.Fatalf("ReadBlocks returned %+v, want %+v", *blocks[0], *want)
	}

	allBlocks, err := client.ReadAllBlocks(ctx)
	if err != nil {
		t.Fatalf("ReadAllBlocks returned error: %v", err)
	}
	if len(allBlocks) != 1 {
		t.Fatalf("ReadAllBlocks returned %d blocks, want 1", len(allBlocks))
	}

	if err := client.DeleteAllBlocks(ctx); err != nil {
		t.Fatalf("DeleteAllBlocks returned error: %v", err)
	}
	afterDelete, err := client.ReadAllBlocks(ctx)
	if err != nil {
		t.Fatalf("ReadAllBlocks after delete returned error: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("ReadAllBlocks after delete returned %d blocks, want 0", len(afterDelete))
	}
}
