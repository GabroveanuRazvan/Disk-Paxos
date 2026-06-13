package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid config",
			cfg: Config{
				ProcessorCount: 3,
				DiskCount:      3,
				DiskAddresses:  []string{"localhost:6379", "localhost:6380", "localhost:6381"},
				Retries:        2,
			},
		},
		{
			name: "disk count mismatch",
			cfg: Config{
				ProcessorCount: 3,
				DiskCount:      3,
				DiskAddresses:  []string{"localhost:6379"},
				Retries:        2,
			},
			wantErr: "inconsistent disk count",
		},
		{
			name: "duplicate disk address",
			cfg: Config{
				ProcessorCount: 3,
				DiskCount:      3,
				DiskAddresses:  []string{"localhost:6379", "localhost:6379", "localhost:6381"},
				Retries:        2,
			},
			wantErr: "duplicate disk address",
		},
		{
			name: "invalid retries",
			cfg: Config{
				ProcessorCount: 3,
				DiskCount:      3,
				DiskAddresses:  []string{"localhost:6379", "localhost:6380", "localhost:6381"},
				Retries:        0,
			},
			wantErr: "invalid retries",
		},
		{
			name: "even disk count",
			cfg: Config{
				ProcessorCount: 3,
				DiskCount:      2,
				DiskAddresses:  []string{"localhost:6379", "localhost:6380"},
				Retries:        2,
			},
			wantErr: "even number of disks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate returned nil error, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
