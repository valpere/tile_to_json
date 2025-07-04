// internal/output/types.go - Output handling types
package output

import (
	"fmt"
	"github.com/valpere/tile_to_json/internal/tile"
	"io"
	"time"
)

// Format represents different output formats supported by the application
type Format string

const (
	FormatGeoJSON Format = "geojson"
	FormatJSON    Format = "json"
	FormatCustom  Format = "custom"
)

// OutputConfig represents configuration for output handling
type OutputConfig struct {
	Format      Format
	Destination string
	Pretty      bool
	Compression bool
	Template    string
	Metadata    bool
	SingleFile  bool
}

// Writer defines the interface for writing processed tiles to various destinations
type Writer interface {
	Write(tile *tile.ProcessedTile) error
	WriteBatch(tiles []*tile.ProcessedTile) error
	Close() error
}

// Formatter defines the interface for formatting processed tiles into different output formats
type Formatter interface {
	Format(tile *tile.ProcessedTile) ([]byte, error)
	FormatBatch(tiles []*tile.ProcessedTile) ([]byte, error)
	ContentType() string
}

// Destination represents an output destination (file, stdout, etc.)
type Destination interface {
	io.WriteCloser
	Name() string
	Size() int64
}

// WriteResult represents the result of a write operation
type WriteResult struct {
	BytesWritten int64
	Duration     time.Duration
	Error        error
}

// BatchWriteResult represents the result of a batch write operation
type BatchWriteResult struct {
	TotalTiles   int
	SuccessTiles int
	FailedTiles  int
	BytesWritten int64
	Duration     time.Duration
	Errors       []error
}

// WriterConfig contains configuration for creating writers
type WriterConfig struct {
	Format      Format
	Pretty      bool
	Compression bool
	BaseDir     string
	Template    string
	Metadata    bool
}

// FormatterConfig contains configuration for creating formatters
type FormatterConfig struct {
	Format       Format
	Pretty       bool
	IncludeStats bool
	Template     string
}

// NewOutputConfig creates a new output configuration with default values
func NewOutputConfig() *OutputConfig {
	return &OutputConfig{
		Format:      FormatGeoJSON,
		Pretty:      true,
		Compression: false,
		Metadata:    true,
		SingleFile:  false,
	}
}

// Validate validates the output configuration
func (c *OutputConfig) Validate() error {
	validFormats := []Format{FormatGeoJSON, FormatJSON, FormatCustom}
	for _, format := range validFormats {
		if c.Format == format {
			return nil
		}
	}
	return fmt.Errorf("invalid output format: %s", c.Format)
}

// String returns a string representation of the format
func (f Format) String() string {
	return string(f)
}

// IsValid checks if the format is supported
func (f Format) IsValid() bool {
	switch f {
	case FormatGeoJSON, FormatJSON, FormatCustom:
		return true
	default:
		return false
	}
}
