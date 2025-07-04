// internal/tile/processor.go - Tile processing implementation
package tile

import (
	"fmt"
	"time"

	"github.com/valpere/tile_to_json/pkg/mvt"
)

// MVTProcessor implements the Processor interface for Mapbox Vector Tiles
type MVTProcessor struct {
	converter *mvt.Converter
}

// NewMVTProcessor creates a new processor for Mapbox Vector Tiles
func NewMVTProcessor() *MVTProcessor {
	return &MVTProcessor{
		converter: mvt.NewConverter(),
	}
}

// Process converts a single tile response to processed JSON data
func (p *MVTProcessor) Process(response *TileResponse) (*ProcessedTile, error) {
	start := time.Now()

	coordinate := &TileCoordinate{
		Z: response.Request.Z,
		X: response.Request.X,
		Y: response.Request.Y,
	}

	// Handle cases where the fetch failed
	if response.Error != nil {
		return &ProcessedTile{
			Coordinate: coordinate,
			Error:      fmt.Errorf("tile fetch failed: %w", response.Error),
		}, response.Error
	}

	// Validate that we have data to process
	if len(response.Data) == 0 {
		return &ProcessedTile{
			Coordinate: coordinate,
			Error:      fmt.Errorf("empty tile data received"),
		}, fmt.Errorf("empty tile data for tile %s", coordinate.String())
	}

	// Convert the MVT data to GeoJSON format
	geojson, metadata, err := p.converter.Convert(
		response.Data,
		response.Request.Z,
		response.Request.X,
		response.Request.Y,
	)
	if err != nil {
		return &ProcessedTile{
			Coordinate: coordinate,
			Error:      fmt.Errorf("MVT conversion failed: %w", err),
		}, err
	}

	processTime := time.Since(start)

	// Build the tile metadata
	tileMetadata := &TileMetadata{
		Layers:       metadata.Layers,
		FeatureCount: metadata.FeatureCount,
		Size:         len(response.Data),
		ProcessTime:  processTime,
		Version:      metadata.Version,
		Extent:       metadata.Extent,
		Compressed:   isCompressed(response.Headers),
	}

	return &ProcessedTile{
		Coordinate: coordinate,
		Data:       geojson,
		Metadata:   tileMetadata,
	}, nil
}

// ProcessBatch processes multiple tile responses concurrently
func (p *MVTProcessor) ProcessBatch(responses []*TileResponse) ([]*ProcessedTile, error) {
	results := make([]*ProcessedTile, len(responses))

	// Process each response
	for i, response := range responses {
		processed, err := p.Process(response)
		if err != nil {
			// For batch processing, we include errors in the results
			// rather than failing the entire batch
			processed = &ProcessedTile{
				Coordinate: &TileCoordinate{
					Z: response.Request.Z,
					X: response.Request.X,
					Y: response.Request.Y,
				},
				Error: err,
			}
		}
		results[i] = processed
	}

	return results, nil
}

// isCompressed checks if the tile data was compressed based on response headers
func isCompressed(headers map[string][]string) bool {
	if contentEncoding, exists := headers["Content-Encoding"]; exists {
		for _, encoding := range contentEncoding {
			if encoding == "gzip" || encoding == "deflate" {
				return true
			}
		}
	}
	return false
}

// ValidateCoordinates ensures tile coordinates are within valid bounds
func ValidateCoordinates(z, x, y int) error {
	if z < 0 || z > 22 {
		return fmt.Errorf("invalid zoom level %d: must be between 0 and 22", z)
	}

	maxTile := 1 << uint(z)
	if x < 0 || x >= maxTile {
		return fmt.Errorf("invalid x coordinate %d for zoom %d: must be between 0 and %d", x, z, maxTile-1)
	}

	if y < 0 || y >= maxTile {
		return fmt.Errorf("invalid y coordinate %d for zoom %d: must be between 0 and %d", y, z, maxTile-1)
	}

	return nil
}
