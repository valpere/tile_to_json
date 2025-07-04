// internal/tile/local_fetcher.go - Local file fetching implementation
package tile

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/valpere/tile_to_json/internal"
	"github.com/valpere/tile_to_json/internal/config"
)

// LocalFetcher implements the Fetcher interface for local file system access
type LocalFetcher struct {
	config *config.LocalConfig
}

// NewLocalFetcher creates a new local file fetcher
func NewLocalFetcher(cfg *config.Config) *LocalFetcher {
	return &LocalFetcher{
		config: &cfg.Local,
	}
}

// Fetch retrieves a tile from the local file system
func (f *LocalFetcher) Fetch(request *TileRequest) (*TileResponse, error) {
	start := time.Now()

	// Build file path from request
	filePath, err := f.buildFilePath(request)
	if err != nil {
		return &TileResponse{
			Request: request,
			Error:   internal.NewError(internal.ErrorCodeValidation, "failed to build file path", err),
		}, err
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			notFoundErr := internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("tile file not found: %s", filePath), err)
			return &TileResponse{
				Request:   request,
				FetchTime: time.Since(start),
				Error:     notFoundErr,
			}, notFoundErr
		}
		accessErr := internal.NewError(internal.ErrorCodeFileSystem, fmt.Sprintf("cannot access tile file: %s", filePath), err)
		return &TileResponse{
			Request:   request,
			FetchTime: time.Since(start),
			Error:     accessErr,
		}, accessErr
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		typeErr := internal.NewError(internal.ErrorCodeValidation, fmt.Sprintf("path is not a regular file: %s", filePath), nil)
		return &TileResponse{
			Request:   request,
			FetchTime: time.Since(start),
			Error:     typeErr,
		}, typeErr
	}

	// Open and read the file
	file, err := os.Open(filePath)
	if err != nil {
		openErr := internal.NewError(internal.ErrorCodeFileSystem, fmt.Sprintf("failed to open tile file: %s", filePath), err)
		return &TileResponse{
			Request:   request,
			FetchTime: time.Since(start),
			Error:     openErr,
		}, openErr
	}
	defer file.Close()

	// Handle compressed files
	var reader io.Reader = file
	isCompressed := f.isCompressedFile(filePath)
	if isCompressed {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			compressErr := internal.NewError(internal.ErrorCodeProcessing, fmt.Sprintf("failed to create gzip reader for: %s", filePath), err)
			return &TileResponse{
				Request:   request,
				FetchTime: time.Since(start),
				Error:     compressErr,
			}, compressErr
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read file content
	data, err := io.ReadAll(reader)
	if err != nil {
		readErr := internal.NewError(internal.ErrorCodeFileSystem, fmt.Sprintf("failed to read tile file: %s", filePath), err)
		return &TileResponse{
			Request:   request,
			FetchTime: time.Since(start),
			Error:     readErr,
		}, readErr
	}

	// Create successful response
	response := &TileResponse{
		Request:    request,
		Data:       data,
		StatusCode: 200, // Simulate HTTP 200 OK for consistency
		Size:       len(data),
		FetchTime:  time.Since(start),
	}

	// Add pseudo-headers for consistency with HTTP fetcher
	response.Headers = make(map[string][]string)
	response.Headers["Content-Type"] = []string{"application/x-protobuf"}
	response.Headers["Content-Length"] = []string{fmt.Sprintf("%d", len(data))}
	if isCompressed {
		response.Headers["Content-Encoding"] = []string{"gzip"}
	}

	return response, nil
}

// FetchWithRetry implements retry logic for local file access (mainly for consistency)
func (f *LocalFetcher) FetchWithRetry(request *TileRequest) (*TileResponse, error) {
	// For local files, retry doesn't make much sense unless it's a temporary
	// file system issue, but we implement it for interface consistency
	maxRetries := 3
	var lastResponse *TileResponse
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Brief delay for potential transient file system issues
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}

		response, err := f.Fetch(request)
		if err == nil {
			return response, nil
		}

		lastResponse = response
		lastErr = err

		// Don't retry on certain error types
		if !f.shouldRetry(response, err) {
			break
		}
	}

	return lastResponse, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// buildFilePath constructs the file path from tile request coordinates
func (f *LocalFetcher) buildFilePath(request *TileRequest) (string, error) {
	if request.URL != "" {
		// If URL is provided, treat it as a direct file path
		if filepath.IsAbs(request.URL) {
			return request.URL, nil
		}
		// Relative path - combine with base path
		return filepath.Join(f.config.BasePath, request.URL), nil
	}

	// Build path from coordinates using template
	if f.config.BasePath == "" {
		return "", fmt.Errorf("base_path is required for coordinate-based file paths")
	}

	// Validate coordinates
	if err := ValidateCoordinates(request.Z, request.X, request.Y); err != nil {
		return "", fmt.Errorf("invalid coordinates: %w", err)
	}

	// Determine file extension
	extension := f.config.Extension
	if f.config.Compressed {
		extension += ".gz"
	}

	// Build path: {base_path}/{z}/{x}/{y}.mvt
	filePath := filepath.Join(
		f.config.BasePath,
		fmt.Sprintf("%d", request.Z),
		fmt.Sprintf("%d", request.X),
		fmt.Sprintf("%d%s", request.Y, extension),
	)

	return filePath, nil
}

// isCompressedFile determines if a file is compressed based on its extension
func (f *LocalFetcher) isCompressedFile(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".gz")
}

// shouldRetry determines if a failed local file access should be retried
func (f *LocalFetcher) shouldRetry(response *TileResponse, err error) bool {
	if response == nil {
		return true // Network-like errors might be transient
	}

	// Don't retry on file not found or permission errors
	if appErr, ok := err.(*internal.Error); ok {
		switch appErr.Code {
		case internal.ErrorCodeNotFound, internal.ErrorCodePermission, internal.ErrorCodeValidation:
			return false
		}
	}

	// Retry on file system errors that might be transient
	return true
}

// ListAvailableTiles scans the local directory structure to find available tiles
func (f *LocalFetcher) ListAvailableTiles() ([]*TileCoordinate, error) {
	if f.config.BasePath == "" {
		return nil, fmt.Errorf("base_path is required for tile listing")
	}

	var tiles []*TileCoordinate

	// Walk through the directory structure
	err := filepath.Walk(f.config.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Parse coordinates from file path
		coords, err := f.parseCoordinatesFromPath(path)
		if err != nil {
			// Skip files that don't match the expected pattern
			return nil
		}

		tiles = append(tiles, coords)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan tile directory: %w", err)
	}

	return tiles, nil
}

// parseCoordinatesFromPath extracts tile coordinates from a file path
func (f *LocalFetcher) parseCoordinatesFromPath(filePath string) (*TileCoordinate, error) {
	// Get relative path from base path
	relPath, err := filepath.Rel(f.config.BasePath, filePath)
	if err != nil {
		return nil, err
	}

	// Split path into components
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid path structure: %s", relPath)
	}

	// Parse Z coordinate (directory)
	var z int
	if _, err := fmt.Sscanf(parts[0], "%d", &z); err != nil {
		return nil, fmt.Errorf("invalid Z coordinate: %s", parts[0])
	}

	// Parse X coordinate (directory)
	var x int
	if _, err := fmt.Sscanf(parts[1], "%d", &x); err != nil {
		return nil, fmt.Errorf("invalid X coordinate: %s", parts[1])
	}

	// Parse Y coordinate (filename)
	filename := parts[2]
	// Remove extension(s)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) // Remove .gz if present
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) // Remove .mvt

	var y int
	if _, err := fmt.Sscanf(filename, "%d", &y); err != nil {
		return nil, fmt.Errorf("invalid Y coordinate: %s", filename)
	}

	return &TileCoordinate{Z: z, X: x, Y: y}, nil
}

// ValidateTileExists checks if a specific tile exists in the local file system
func (f *LocalFetcher) ValidateTileExists(z, x, y int) error {
	request := &TileRequest{Z: z, X: x, Y: y}
	filePath, err := f.buildFilePath(request)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return internal.NewError(internal.ErrorCodeNotFound, fmt.Sprintf("tile %d/%d/%d not found", z, x, y), err)
		}
		return internal.NewError(internal.ErrorCodeFileSystem, fmt.Sprintf("cannot access tile %d/%d/%d", z, x, y), err)
	}

	return nil
}

// GetTileInfo returns information about a local tile file
func (f *LocalFetcher) GetTileInfo(z, x, y int) (*TileFileInfo, error) {
	request := &TileRequest{Z: z, X: x, Y: y}
	filePath, err := f.buildFilePath(request)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return &TileFileInfo{
		Path:         filePath,
		Size:         fileInfo.Size(),
		ModTime:      fileInfo.ModTime(),
		IsCompressed: f.isCompressedFile(filePath),
		Exists:       true,
	}, nil
}

// TileFileInfo contains information about a local tile file
type TileFileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsCompressed bool      `json:"is_compressed"`
	Exists       bool      `json:"exists"`
}
