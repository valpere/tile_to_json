// internal/config/validation.go - Configuration validation
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Validate validates the configuration structure and values
func Validate(config *Config) error {
	if err := validateSource(&config.Source); err != nil {
		return fmt.Errorf("source configuration invalid: %w", err)
	}

	if err := validateServer(&config.Server); err != nil {
		return fmt.Errorf("server configuration invalid: %w", err)
	}

	if err := validateLocal(&config.Local); err != nil {
		return fmt.Errorf("local configuration invalid: %w", err)
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

	// Cross-configuration validation
	if err := validateSourceCombination(config); err != nil {
		return fmt.Errorf("source configuration combination invalid: %w", err)
	}

	return nil
}

// validateSource validates source configuration parameters
func validateSource(config *SourceConfig) error {
	validTypes := []string{"http", "local", "auto"}
	if !contains(validTypes, config.Type) {
		return fmt.Errorf("invalid source type: %s, must be one of %v", config.Type, validTypes)
	}

	validDefaultTypes := []string{"http", "local"}
	if !contains(validDefaultTypes, config.DefaultType) {
		return fmt.Errorf("invalid default source type: %s, must be one of %v", config.DefaultType, validDefaultTypes)
	}

	return nil
}

// validateServer validates server configuration parameters
func validateServer(config *ServerConfig) error {
	// Server configuration is optional if using local files
	if config.BaseURL == "" {
		return nil // Allow empty server config for local-only usage
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
		return fmt.Errorf("url_template is required when base_url is specified")
	}

	return nil
}

// validateLocal validates local file configuration parameters
func validateLocal(config *LocalConfig) error {
	// Local configuration is optional if using HTTP sources
	if config.BasePath == "" {
		return nil // Allow empty local config for HTTP-only usage
	}

	// Check if base path exists and is accessible
	if _, err := os.Stat(config.BasePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("base_path does not exist: %s", config.BasePath)
		}
		return fmt.Errorf("base_path is not accessible: %w", err)
	}

	// Validate path template
	if config.PathTemplate == "" {
		return fmt.Errorf("path_template is required when base_path is specified")
	}

	// Check if path template contains required placeholders
	requiredPlaceholders := []string{"{z}", "{x}", "{y}"}
	for _, placeholder := range requiredPlaceholders {
		if !strings.Contains(config.PathTemplate, placeholder) {
			return fmt.Errorf("path_template must contain %s placeholder", placeholder)
		}
	}

	// Validate file extension
	if config.Extension == "" {
		return fmt.Errorf("extension cannot be empty")
	}

	if !strings.HasPrefix(config.Extension, ".") {
		return fmt.Errorf("extension must start with a dot")
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

// validateSourceCombination validates that source configuration combinations make sense
func validateSourceCombination(config *Config) error {
	sourceType := config.DetermineSourceType()

	switch sourceType {
	case "http":
		if config.Server.BaseURL == "" {
			return fmt.Errorf("base_url is required for HTTP source type")
		}
	case "local":
		if config.Local.BasePath == "" {
			return fmt.Errorf("base_path is required for local source type")
		}
		// Validate that the base path is a directory
		if info, err := os.Stat(config.Local.BasePath); err != nil {
			return fmt.Errorf("cannot access base_path: %w", err)
		} else if !info.IsDir() {
			return fmt.Errorf("base_path must be a directory")
		}
	default:
		return fmt.Errorf("invalid source type determined: %s", sourceType)
	}

	return nil
}

// validateLocalTileExists checks if a specific local tile file exists
func ValidateLocalTileExists(config *Config, z, x, y int) error {
	if config.DetermineSourceType() != "local" {
		return nil // Skip validation for non-local sources
	}

	tilePath := config.GetTilePath(z, x, y)
	if tilePath == "" {
		return fmt.Errorf("cannot generate tile path for coordinates %d/%d/%d", z, x, y)
	}

	if _, err := os.Stat(tilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("tile file does not exist: %s", tilePath)
		}
		return fmt.Errorf("cannot access tile file: %w", err)
	}

	return nil
}

// ValidateLocalTileDirectory checks if the local tile directory structure is valid
func ValidateLocalTileDirectory(config *Config) error {
	if config.DetermineSourceType() != "local" {
		return nil // Skip validation for non-local sources
	}

	basePath := config.Local.BasePath
	if basePath == "" {
		return fmt.Errorf("base_path is required for local tile validation")
	}

	// Check if base directory exists and is readable
	info, err := os.Stat(basePath)
	if err != nil {
		return fmt.Errorf("cannot access base path %s: %w", basePath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("base path %s is not a directory", basePath)
	}

	// Check if we can read the directory
	if _, err := os.ReadDir(basePath); err != nil {
		return fmt.Errorf("cannot read base directory %s: %w", basePath, err)
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
