package main

import (
	"context"
	"disk-paxos/internal/config"
	"disk-paxos/internal/disk"
	"fmt"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panic(err)
	}

	ctx := context.Background()
	for _, addr := range cfg.DiskAddresses {
		client := disk.NewClient(cfg, addr)
		if err = client.DeleteAllBlocks(ctx); err != nil {
			fmt.Errorf("%w", err)
		}
		client.Close()
	}

	fmt.Println("Successfully cleared all disks")
}
