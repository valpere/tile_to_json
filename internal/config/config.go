// internal/config/config.go - Configuration management
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"github.com/valpere/tile_to_json/internal"
)

// Config represents the complete application configuration
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Output  OutputConfig  `mapstructure:"output"`
	Batch   BatchConfig   `mapstructure:"batch"`
	Network NetworkConfig `mapstructure:"network"`
	Logging LoggingConfig `mapstructure:"logging"`
}

// ServerConfig contains tile server configuration
type ServerConfig struct {
	BaseURL     string            `mapstructure:"base_url"`
	APIKey      string            `mapstructure:"api_key"`
	Headers     map[string]string `mapstructure:"headers"`
	Timeout     time.Duration     `mapstructure:"timeout"`
	MaxRetries  int               `mapstructure:"max_retries"`
	URLTemplate string            `mapstructure:"url_template"`
}

// OutputConfig contains output formatting configuration
type OutputConfig struct {
	Format      string `mapstructure:"format"`
	Directory   string `mapstructure:"directory"`
	Filename    string `mapstructure:"filename"`
	Compression bool   `mapstructure:"compression"`
	Pretty      bool   `mapstructure:"pretty"`
	Stdout      bool   `mapstructure:"stdout"`
}

// BatchConfig contains batch processing configuration
type BatchConfig struct {
	Concurrency int           `mapstructure:"concurrency"`
	ChunkSize   int           `mapstructure:"chunk_size"`
	Timeout     time.Duration `mapstructure:"timeout"`
	Resume      bool          `mapstructure:"resume"`
	FailOnError bool          `mapstructure:"fail_on_error"`
}

// NetworkConfig contains network-related configuration
type NetworkConfig struct {
	ProxyURL         string        `mapstructure:"proxy_url"`
	UserAgent        string        `mapstructure:"user_agent"`
	KeepAlive        time.Duration `mapstructure:"keep_alive"`
	MaxIdleConns     int           `mapstructure:"max_idle_conns"`
	IdleConnTimeout  time.Duration `mapstructure:"idle_conn_timeout"`
	DisableKeepAlive bool          `mapstructure:"disable_keep_alive"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	Verbose  bool   `mapstructure:"verbose"`
	Progress bool   `mapstructure:"progress"`
}

// Load loads configuration from various sources
func Load() (*Config, error) {
	// Set default values
	setDefaults()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := Validate(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults configures default values for all configuration options
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.timeout", 30*time.Second)
	viper.SetDefault("server.max_retries", 3)
	viper.SetDefault("server.url_template", "{base_url}/{z}/{x}/{y}.mvt")

	// Output defaults
	viper.SetDefault("output.format", "geojson")
	viper.SetDefault("output.pretty", true)
	viper.SetDefault("output.compression", false)
	viper.SetDefault("output.stdout", false)

	// Batch defaults
	viper.SetDefault("batch.concurrency", 10)
	viper.SetDefault("batch.chunk_size", 100)
	viper.SetDefault("batch.timeout", 5*time.Minute)
	viper.SetDefault("batch.resume", false)
	viper.SetDefault("batch.fail_on_error", false)

	// Network defaults
	viper.SetDefault("network.user_agent", "TileToJson/1.0")
	viper.SetDefault("network.keep_alive", 30*time.Second)
	viper.SetDefault("network.max_idle_conns", 100)
	viper.SetDefault("network.idle_conn_timeout", 90*time.Second)
	viper.SetDefault("network.disable_keep_alive", false)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "stderr")
	viper.SetDefault("logging.verbose", false)
	viper.SetDefault("logging.progress", true)
}

// ToApplicationConfig converts Config to internal.ApplicationConfig
func (c *Config) ToApplicationConfig() *internal.ApplicationConfig {
	return &internal.ApplicationConfig{
		LogLevel:       c.Logging.Level,
		MaxConcurrency: c.Batch.Concurrency,
		RequestTimeout: c.Server.Timeout,
		RetryAttempts:  c.Server.MaxRetries,
		RetryDelay:     time.Second,
	}
}

// GetTileURL builds a tile URL using the configured template
func (c *Config) GetTileURL(z, x, y int) string {
	url := c.Server.URLTemplate
	url = viper.GetString("server.base_url") + "/" + fmt.Sprintf("%d/%d/%d.mvt", z, x, y)
	
	if c.Server.BaseURL != "" {
		// Simple template replacement for now
		return fmt.Sprintf("%s/%d/%d/%d.mvt", c.Server.BaseURL, z, x, y)
	}
	
	return url
}
