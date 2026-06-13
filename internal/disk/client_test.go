package disk

import (
	"context"
	"testing"
)

func TestReadBlocksWithNoIDsReturnsEmptyResult(t *testing.T) {
	client := &Client{}

	blocks, err := client.ReadBlocks(context.Background())
	if err != nil {
		t.Fatalf("ReadBlocks returned error: %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("ReadBlocks returned %d blocks, want 0", len(blocks))
	}
}
