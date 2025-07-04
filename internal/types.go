// internal/types.go - Common types for internal packages
package internal

import (
	"context"
	"time"
)

// SourceType represents the type of tile data source
type SourceType string

const (
	SourceTypeHTTP  SourceType = "http"
	SourceTypeLocal SourceType = "local"
)

// ApplicationConfig represents the global application configuration
type ApplicationConfig struct {
	LogLevel       string
	MaxConcurrency int
	RequestTimeout time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
	SourceType     SourceType
}

// ProcessingStats represents metrics for processing operations
type ProcessingStats struct {
	TotalTiles     int64
	ProcessedTiles int64
	FailedTiles    int64
	StartTime      time.Time
	EndTime        time.Time
	Throughput     float64
}

// ProcessingContext extends context with application-specific data
type ProcessingContext struct {
	context.Context
	Config *ApplicationConfig
	Stats  *ProcessingStats
}

// Error represents application-specific errors
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// NewError creates a new application error
func NewError(code, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// ErrorCode constants for common error types
const (
	ErrorCodeNetwork    = "NETWORK_ERROR"
	ErrorCodeProcessing = "PROCESSING_ERROR"
	ErrorCodeValidation = "VALIDATION_ERROR"
	ErrorCodeConfig     = "CONFIG_ERROR"
	ErrorCodeNotFound   = "NOT_FOUND"
	ErrorCodeTimeout    = "TIMEOUT_ERROR"
	ErrorCodeFileSystem = "FILESYSTEM_ERROR"
	ErrorCodePermission = "PERMISSION_ERROR"
)
