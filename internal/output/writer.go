// internal/output/writer.go - Output writing implementation
package output

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/valpere/tile_to_json/internal/tile"
)

// FileWriter writes output to files with optional compression
type FileWriter struct {
	formatter   Formatter
	destination Destination
	config      *WriterConfig
}

// NewFileWriter creates a new file-based writer
func NewFileWriter(config *WriterConfig, destination string) (*FileWriter, error) {
	formatter, err := NewFormatter(&FormatterConfig{
		Format:       config.Format,
		Pretty:       config.Pretty,
		IncludeStats: config.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create formatter: %w", err)
	}

	dest, err := newFileDestination(destination, config.Compression)
	if err != nil {
		return nil, fmt.Errorf("failed to create file destination: %w", err)
	}

	return &FileWriter{
		formatter:   formatter,
		destination: dest,
		config:      config,
	}, nil
}

// Write writes a single processed tile to the output destination
func (w *FileWriter) Write(tile *tile.ProcessedTile) error {
	data, err := w.formatter.Format(tile)
	if err != nil {
		return fmt.Errorf("formatting failed: %w", err)
	}

	_, err = w.destination.Write(data)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

// WriteBatch writes multiple processed tiles as a batch operation
func (w *FileWriter) WriteBatch(tiles []*tile.ProcessedTile) error {
	data, err := w.formatter.FormatBatch(tiles)
	if err != nil {
		return fmt.Errorf("batch formatting failed: %w", err)
	}

	_, err = w.destination.Write(data)
	if err != nil {
		return fmt.Errorf("batch write failed: %w", err)
	}

	return nil
}

// Close closes the writer and underlying destination
func (w *FileWriter) Close() error {
	return w.destination.Close()
}

// StdoutWriter writes output to standard output
type StdoutWriter struct {
	formatter Formatter
}

// NewStdoutWriter creates a new stdout-based writer
func NewStdoutWriter(format Format, pretty bool) (*StdoutWriter, error) {
	formatter, err := NewFormatter(&FormatterConfig{
		Format:       format,
		Pretty:       pretty,
		IncludeStats: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create formatter: %w", err)
	}

	return &StdoutWriter{formatter: formatter}, nil
}

// Write writes a single tile to stdout
func (w *StdoutWriter) Write(tile *tile.ProcessedTile) error {
	data, err := w.formatter.Format(tile)
	if err != nil {
		return fmt.Errorf("formatting failed: %w", err)
	}

	_, err = os.Stdout.Write(data)
	if err != nil {
		return fmt.Errorf("write to stdout failed: %w", err)
	}

	// Add newline for readability
	_, err = os.Stdout.Write([]byte("\n"))
	return err
}

// WriteBatch writes multiple tiles to stdout
func (w *StdoutWriter) WriteBatch(tiles []*tile.ProcessedTile) error {
	data, err := w.formatter.FormatBatch(tiles)
	if err != nil {
		return fmt.Errorf("batch formatting failed: %w", err)
	}

	_, err = os.Stdout.Write(data)
	if err != nil {
		return fmt.Errorf("batch write to stdout failed: %w", err)
	}

	// Add newline for readability
	_, err = os.Stdout.Write([]byte("\n"))
	return err
}

// Close is a no-op for stdout writer
func (w *StdoutWriter) Close() error {
	return nil
}

// MultiFileWriter writes each tile to a separate file
type MultiFileWriter struct {
	formatter Formatter
	baseDir   string
	config    *WriterConfig
}

// NewMultiFileWriter creates a writer that outputs each tile to a separate file
func NewMultiFileWriter(config *WriterConfig, baseDir string) (*MultiFileWriter, error) {
	formatter, err := NewFormatter(&FormatterConfig{
		Format:       config.Format,
		Pretty:       config.Pretty,
		IncludeStats: config.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create formatter: %w", err)
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &MultiFileWriter{
		formatter: formatter,
		baseDir:   baseDir,
		config:    config,
	}, nil
}

// Write writes a single tile to its own file
func (w *MultiFileWriter) Write(tile *tile.ProcessedTile) error {
	filename := w.generateFilename(tile.Coordinate)
	filepath := filepath.Join(w.baseDir, filename)

	// Ensure subdirectory exists
	if err := os.MkdirAll(filepath.Dir(filepath), 0755); err != nil {
		return fmt.Errorf("failed to create subdirectory: %w", err)
	}

	dest, err := newFileDestination(filepath, w.config.Compression)
	if err != nil {
		return fmt.Errorf("failed to create file destination: %w", err)
	}
	defer dest.Close()

	data, err := w.formatter.Format(tile)
	if err != nil {
		return fmt.Errorf("formatting failed: %w", err)
	}

	_, err = dest.Write(data)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

// WriteBatch writes each tile in the batch to separate files
func (w *MultiFileWriter) WriteBatch(tiles []*tile.ProcessedTile) error {
	for _, tile := range tiles {
		if err := w.Write(tile); err != nil {
			return fmt.Errorf("failed to write tile %s: %w", tile.Coordinate.String(), err)
		}
	}
	return nil
}

// Close is a no-op for multi-file writer
func (w *MultiFileWriter) Close() error {
	return nil
}

// generateFilename creates a filename for a tile based on its coordinates
func (w *MultiFileWriter) generateFilename(coord *tile.TileCoordinate) string {
	ext := w.getFileExtension()
	if w.config.Compression {
		ext += ".gz"
	}
	return fmt.Sprintf("%d/%d/%d%s", coord.Z, coord.X, coord.Y, ext)
}

// getFileExtension returns the appropriate file extension for the format
func (w *MultiFileWriter) getFileExtension() string {
	switch w.config.Format {
	case FormatGeoJSON:
		return ".geojson"
	case FormatJSON:
		return ".json"
	default:
		return ".json"
	}
}

// fileDestination implements the Destination interface for file output
type fileDestination struct {
	file   *os.File
	writer io.WriteCloser
	name   string
	size   int64
}

// newFileDestination creates a new file destination with optional compression
func newFileDestination(path string, compression bool) (*fileDestination, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	var writer io.WriteCloser = file
	if compression {
		// Add .gz extension if not already present
		if !strings.HasSuffix(path, ".gz") {
			file.Close()
			newPath := path + ".gz"
			file, err = os.Create(newPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create compressed file: %w", err)
			}
			path = newPath
		}
		writer = gzip.NewWriter(file)
	}

	return &fileDestination{
		file:   file,
		writer: writer,
		name:   path,
		size:   0,
	}, nil
}

// Write implements io.Writer
func (d *fileDestination) Write(p []byte) (n int, err error) {
	n, err = d.writer.Write(p)
	d.size += int64(n)
	return n, err
}

// Close implements io.Closer
func (d *fileDestination) Close() error {
	if d.writer != d.file {
		if err := d.writer.Close(); err != nil {
			d.file.Close()
			return err
		}
	}
	return d.file.Close()
}

// Name returns the destination file path
func (d *fileDestination) Name() string {
	return d.name
}

// Size returns the number of bytes written
func (d *fileDestination) Size() int64 {
	return d.size
}

// NewWriter creates the appropriate writer based on configuration
func NewWriter(config *WriterConfig, destination string, multiFile bool) (Writer, error) {
	if destination == "" || destination == "-" {
		return NewStdoutWriter(config.Format, config.Pretty)
	}

	if multiFile {
		return NewMultiFileWriter(config, destination)
	}

	return NewFileWriter(config, destination)
}
