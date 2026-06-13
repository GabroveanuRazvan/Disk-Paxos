package block

import "testing"

func TestBlockJSONRoundTrip(t *testing.T) {
	original := &Block{
		Mbal: 7,
		Bal:  4,
		Inp:  "value",
	}

	var decoded Block
	decoded.FromJSON(original.JSON())

	if decoded != *original {
		t.Fatalf("decoded block mismatch: got %+v, want %+v", decoded, *original)
	}
}

func TestBlockFromJSONKeepsExistingValueOnInvalidJSON(t *testing.T) {
	blk := &Block{
		Mbal: 1,
		Bal:  1,
		Inp:  "accepted",
	}

	blk.FromJSON("{")

	want := &Block{
		Mbal: 1,
		Bal:  1,
		Inp:  "accepted",
	}
	if *blk != *want {
		t.Fatalf("block changed after invalid json: got %+v, want %+v", *blk, *want)
	}
}
