// internal/config/validation.go - Configuration validation
package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Validate validates the configuration structure and values
func Validate(config *Config) error {
	if err := validateServer(&config.Server); err != nil {
		return fmt.Errorf("server configuration invalid: %w", err)
	}

	if err := validateOutput(&config.Output); err != nil {
		return fmt.Errorf("output configuration invalid: %w", err)
	}

	if err := validateBatch(&config.Batch); err != nil {
		return fmt.Errorf("batch configuration invalid: %w", err)
	}

	if err := validateNetwork(&config.Network); err != nil {
		return fmt.Errorf("network configuration invalid: %w", err)
	}

	if err := validateLogging(&config.Logging); err != nil {
		return fmt.Errorf("logging configuration invalid: %w", err)
	}

	return nil
}

// validateServer validates server configuration parameters
func validateServer(config *ServerConfig) error {
	if config.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	if _, err := url.Parse(config.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	if config.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}

	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if config.URLTemplate == "" {
		return fmt.Errorf("url_template is required")
	}

	return nil
}

// validateOutput validates output configuration parameters
func validateOutput(config *OutputConfig) error {
	validFormats := []string{"geojson", "json", "custom"}
	if !contains(validFormats, config.Format) {
		return fmt.Errorf("invalid format: %s, must be one of %v", config.Format, validFormats)
	}

	if !config.Stdout && config.Directory == "" {
		return fmt.Errorf("directory is required when not using stdout")
	}

	return nil
}

// validateBatch validates batch processing configuration parameters
func validateBatch(config *BatchConfig) error {
	if config.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}

	if config.Concurrency > 1000 {
		return fmt.Errorf("concurrency must not exceed 1000")
	}

	if config.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be positive")
	}

	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

// validateNetwork validates network configuration parameters
func validateNetwork(config *NetworkConfig) error {
	if config.ProxyURL != "" {
		if _, err := url.Parse(config.ProxyURL); err != nil {
			return fmt.Errorf("invalid proxy_url: %w", err)
		}
	}

	if config.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns must be non-negative")
	}

	if config.UserAgent == "" {
		return fmt.Errorf("user_agent cannot be empty")
	}

	if config.KeepAlive < 0 {
		return fmt.Errorf("keep_alive must be non-negative")
	}

	if config.IdleConnTimeout < 0 {
		return fmt.Errorf("idle_conn_timeout must be non-negative")
	}

	return nil
}

// validateLogging validates logging configuration parameters
func validateLogging(config *LoggingConfig) error {
	validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	if !contains(validLevels, config.Level) {
		return fmt.Errorf("invalid log level: %s, must be one of %v", config.Level, validLevels)
	}

	validFormats := []string{"text", "json"}
	if !contains(validFormats, config.Format) {
		return fmt.Errorf("invalid log format: %s, must be one of %v", config.Format, validFormats)
	}

	validOutputs := []string{"stdout", "stderr", "file"}
	if !contains(validOutputs, config.Output) {
		return fmt.Errorf("invalid log output: %s, must be one of %v", config.Output, validOutputs)
	}

	return nil
}

// contains checks if a string slice contains a specific string (case-insensitive)
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
