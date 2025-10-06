package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Database struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"database"`

	AWS struct {
		MaxConcurrentSessions int `mapstructure:"max_concurrent_sessions"`
	} `mapstructure:"aws"`

	Discovery struct {
		TTL string `mapstructure:"ttl"`
	} `mapstructure:"discovery"`
}

var globalConfig *Config

// InitConfig initializes the application configuration
func InitConfig(cfgFile string) error {
	// Set defaults
	setDefaults()

	// Unmarshal config
	globalConfig = &Config{}
	if err := viper.Unmarshal(globalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths
	if globalConfig.Database.Path == "~/.ssm/database.db" {
		homeDir, _ := os.UserHomeDir()
		globalConfig.Database.Path = filepath.Join(homeDir, ".ssm", "database.db")
	}

	// Set log level
	if viper.GetBool("verbose") {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	return nil
}

// GetConfig returns the global configuration
func GetConfig() *Config {
	return globalConfig
}

// setDefaults sets the default configuration values
func setDefaults() {
	viper.SetDefault("database.path", "~/.ssm/database.db")
	viper.SetDefault("aws.max_concurrent_sessions", 5)

	viper.SetDefault("discovery.ttl", "24h")
}
