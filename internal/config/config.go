package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	ProcessorCount int      `mapstructure:"PROCESSOR_COUNT"`
	DiskCount      int      `mapstructure:"DISK_COUNT"`
	DiskAddresses  []string `mapstructure:"DISK_ADDRESSES"`
	Retries        int      `mapstructure:"RETRIES"`
}

func setDefaults() {
	viper.SetDefault("PROCESSOR_COUNT", 1)
	viper.SetDefault("DISK_COUNT", 3)
	viper.SetDefault("DISK_ADDRESSES", []string{
		"localhost:6379",
		"localhost:6380",
		"localhost:6381",
	})
	viper.SetDefault("RETRIES", 3)
}

func LoadConfig() (*Config, error) {
	setDefaults()
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.DiskCount != len(cfg.DiskAddresses) {
		return fmt.Errorf("inconsistent disk count and address count: %d != %d", cfg.DiskCount, len(cfg.DiskAddresses))
	}

	addrSet := make(map[string]struct{})

	for _, addr := range cfg.DiskAddresses {
		_, exists := addrSet[addr]
		if exists {
			return fmt.Errorf("duplicate disk address: %s", addr)
		}
		addrSet[addr] = struct{}{}
	}

	if cfg.Retries < 1 {
		return fmt.Errorf("invalid retries: %d", cfg.Retries)
	}

	if cfg.DiskCount%2 == 0 {
		return fmt.Errorf("even number of disks: %d", cfg.DiskCount)
	}

	return nil
}
