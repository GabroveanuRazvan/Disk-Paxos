package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	ProcessorCount int `mapstructure:"PROCESSOR_COUNT"`
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

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("PROCESSOR_COUNT", 1)
}
