// internal/tile/types.go - Tile processing types
package tile

import (
	"fmt"
	"net/http"
	"time"
)

// TileRequest represents a request for a specific tile
type TileRequest struct {
	Z       int               `json:"z"`
	X       int               `json:"x"`
	Y       int               `json:"y"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// TileResponse represents the response from a tile server
type TileResponse struct {
	Request    *TileRequest  `json:"request"`
	Data       []byte        `json:"data"`
	Headers    http.Header   `json:"headers"`
	StatusCode int           `json:"status_code"`
	Size       int           `json:"size"`
	FetchTime  time.Duration `json:"fetch_time"`
	Error      error         `json:"error,omitempty"`
}

// TileCoordinate represents a tile coordinate in the tile pyramid
type TileCoordinate struct {
	Z int `json:"z"`
	X int `json:"x"`
	Y int `json:"y"`
}

// TileRange represents a range of tiles to be processed
type TileRange struct {
	MinZ int `json:"min_z"`
	MaxZ int `json:"max_z"`
	MinX int `json:"min_x"`
	MaxX int `json:"max_x"`
	MinY int `json:"min_y"`
	MaxY int `json:"max_y"`
}

// ProcessedTile represents a tile after conversion to JSON format
type ProcessedTile struct {
	Coordinate *TileCoordinate `json:"coordinate"`
	Data       interface{}     `json:"data"`
	Metadata   *TileMetadata   `json:"metadata"`
	Error      error           `json:"error,omitempty"`
}

// TileMetadata contains metadata about the processed tile
type TileMetadata struct {
	Layers       []string      `json:"layers"`
	FeatureCount int           `json:"feature_count"`
	Size         int           `json:"size"`
	ProcessTime  time.Duration `json:"process_time"`
	Version      int           `json:"version"`
	Extent       int           `json:"extent"`
	Compressed   bool          `json:"compressed"`
}

// Fetcher defines the interface for fetching tiles from remote servers
type Fetcher interface {
	Fetch(request *TileRequest) (*TileResponse, error)
	FetchWithRetry(request *TileRequest) (*TileResponse, error)
}

// Processor defines the interface for processing vector tiles
type Processor interface {
	Process(response *TileResponse) (*ProcessedTile, error)
	ProcessBatch(responses []*TileResponse) ([]*ProcessedTile, error)
}

// NewTileRequest creates a new tile request with the specified coordinates
func NewTileRequest(z, x, y int, baseURL string) *TileRequest {
	url := buildTileURL(baseURL, z, x, y)
	return &TileRequest{
		Z:       z,
		X:       x,
		Y:       y,
		URL:     url,
		Headers: make(map[string]string),
	}
}

// NewTileCoordinate creates a new tile coordinate
func NewTileCoordinate(z, x, y int) *TileCoordinate {
	return &TileCoordinate{
		Z: z,
		X: x,
		Y: y,
	}
}

// NewTileRange creates a new tile range
func NewTileRange(minZ, maxZ, minX, maxX, minY, maxY int) *TileRange {
	return &TileRange{
		MinZ: minZ,
		MaxZ: maxZ,
		MinX: minX,
		MaxX: maxX,
		MinY: minY,
		MaxY: maxY,
	}
}

// String returns a string representation of the tile coordinate
func (tc *TileCoordinate) String() string {
	return fmt.Sprintf("%d/%d/%d", tc.Z, tc.X, tc.Y)
}

// Count returns the total number of tiles in the range
func (tr *TileRange) Count() int64 {
	var total int64
	for z := tr.MinZ; z <= tr.MaxZ; z++ {
		xRange := int64(tr.MaxX - tr.MinX + 1)
		yRange := int64(tr.MaxY - tr.MinY + 1)
		total += xRange * yRange
	}
	return total
}

// buildTileURL constructs a tile URL from base URL and coordinates
func buildTileURL(baseURL string, z, x, y int) string {
	return fmt.Sprintf("%s/%d/%d/%d.mvt", baseURL, z, x, y)
}
