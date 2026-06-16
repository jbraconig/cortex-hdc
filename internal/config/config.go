package config

import (
	"github.com/spf13/viper"
)

// Config stores all application configuration
type Config struct {
	Workers     int     `mapstructure:"workers"`
	Threshold   float64 `mapstructure:"threshold"`
	Webhook     string  `mapstructure:"webhook"`
	File        string  `mapstructure:"file"`
	Verbose     bool    `mapstructure:"verbose"`
	MetricsPort int     `mapstructure:"metrics_port"`
}

// LoadConfig reads configuration from environment variables prefixed with CORTEX_*
func LoadConfig() (*Config, error) {
	// Define default values
	viper.SetDefault("workers", 4)
	viper.SetDefault("threshold", 0.65)
	viper.SetDefault("webhook", "")
	viper.SetDefault("file", "")
	viper.SetDefault("verbose", false)
	viper.SetDefault("metrics_port", 9090)

	// Allow reading from environment variables with CORTEX prefix (e.g., CORTEX_WORKERS)
	viper.SetEnvPrefix("CORTEX")
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
