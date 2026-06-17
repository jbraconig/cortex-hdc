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
	InitLogs    string  `mapstructure:"init_logs"`
	Clusters    int     `mapstructure:"clusters"`   // Phase 3.1: 0=single baseline, >=2=multi-cluster
	DecayRate   float64 `mapstructure:"decay_rate"` // Phase 3.3: 0=disabled, 0.001=slow, 0.01=moderate
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
	viper.SetDefault("init_logs", "/data/init-logs/")
	viper.SetDefault("clusters", 0)
	viper.SetDefault("decay_rate", 0.0)

	// Allow reading from environment variables with CORTEX prefix (e.g., CORTEX_WORKERS)
	viper.SetEnvPrefix("CORTEX")
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
