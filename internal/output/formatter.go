// internal/output/formatter.go - Output formatting implementation
package output

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/valpere/tile_to_json/internal/tile"
)

// GeoJSONFormatter formats tiles as GeoJSON FeatureCollection
type GeoJSONFormatter struct {
	pretty       bool
	includeStats bool
}

// NewGeoJSONFormatter creates a new GeoJSON formatter
func NewGeoJSONFormatter(pretty, includeStats bool) *GeoJSONFormatter {
	return &GeoJSONFormatter{
		pretty:       pretty,
		includeStats: includeStats,
	}
}

// Format formats a single processed tile as GeoJSON
func (f *GeoJSONFormatter) Format(tile *tile.ProcessedTile) ([]byte, error) {
	if tile.Error != nil {
		return nil, fmt.Errorf("cannot format tile with error: %w", tile.Error)
	}

	// Extract the GeoJSON data from the tile
	output := tile.Data

	// Add metadata if requested
	if f.includeStats && tile.Metadata != nil {
		if geoJSON, ok := output.(map[string]interface{}); ok {
			geoJSON["_metadata"] = map[string]interface{}{
				"tile_coordinate": tile.Coordinate,
				"layers":          tile.Metadata.Layers,
				"feature_count":   tile.Metadata.FeatureCount,
				"size_bytes":      tile.Metadata.Size,
				"process_time":    tile.Metadata.ProcessTime,
				"version":         tile.Metadata.Version,
				"extent":          tile.Metadata.Extent,
			}
		}
	}

	if f.pretty {
		return json.MarshalIndent(output, "", "  ")
	}
	return json.Marshal(output)
}

// FormatBatch formats multiple tiles as a single GeoJSON FeatureCollection
func (f *GeoJSONFormatter) FormatBatch(tiles []*tile.ProcessedTile) ([]byte, error) {
	collection := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": make([]interface{}, 0),
	}

	var totalFeatures int
	var processedTiles int
	var failedTiles int

	for _, t := range tiles {
		if t.Error != nil {
			failedTiles++
			continue
		}

		processedTiles++

		// Extract features from the tile's GeoJSON data
		if data, ok := t.Data.(map[string]interface{}); ok {
			if features, exists := data["features"]; exists {
				if featureList, ok := features.([]interface{}); ok {
					// Add tile coordinate to each feature if metadata is enabled
					if f.includeStats {
						for _, feature := range featureList {
							if feat, ok := feature.(map[string]interface{}); ok {
								if props, ok := feat["properties"].(map[string]interface{}); ok {
									props["_tile"] = fmt.Sprintf("%d/%d/%d", t.Coordinate.Z, t.Coordinate.X, t.Coordinate.Y)
								}
							}
						}
					}
					collection["features"] = append(collection["features"].([]interface{}), featureList...)
					totalFeatures += len(featureList)
				}
			}
		}
	}

	// Add collection-level metadata
	if f.includeStats {
		collection["_metadata"] = map[string]interface{}{
			"total_tiles":     len(tiles),
			"processed_tiles": processedTiles,
			"failed_tiles":    failedTiles,
			"total_features":  totalFeatures,
			"generated_at":    time.Now().UTC(),
		}
	}

	if f.pretty {
		return json.MarshalIndent(collection, "", "  ")
	}
	return json.Marshal(collection)
}

// ContentType returns the MIME type for GeoJSON
func (f *GeoJSONFormatter) ContentType() string {
	return "application/geo+json"
}

// JSONFormatter formats tiles as structured JSON objects
type JSONFormatter struct {
	pretty       bool
	includeStats bool
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter(pretty, includeStats bool) *JSONFormatter {
	return &JSONFormatter{
		pretty:       pretty,
		includeStats: includeStats,
	}
}

// Format formats a single tile as a JSON object
func (f *JSONFormatter) Format(tile *tile.ProcessedTile) ([]byte, error) {
	output := map[string]interface{}{
		"coordinate": tile.Coordinate,
		"data":       tile.Data,
	}

	if tile.Error != nil {
		output["error"] = tile.Error.Error()
		output["data"] = nil
	}

	if f.includeStats && tile.Metadata != nil {
		output["metadata"] = tile.Metadata
	}

	if f.pretty {
		return json.MarshalIndent(output, "", "  ")
	}
	return json.Marshal(output)
}

// FormatBatch formats multiple tiles as a JSON array
func (f *JSONFormatter) FormatBatch(tiles []*tile.ProcessedTile) ([]byte, error) {
	output := make([]interface{}, 0, len(tiles))

	for _, t := range tiles {
		tileOutput := map[string]interface{}{
			"coordinate": t.Coordinate,
			"data":       t.Data,
		}

		if t.Error != nil {
			tileOutput["error"] = t.Error.Error()
			tileOutput["data"] = nil
		}

		if f.includeStats && t.Metadata != nil {
			tileOutput["metadata"] = t.Metadata
		}

		output = append(output, tileOutput)
	}

	result := map[string]interface{}{
		"tiles": output,
	}

	if f.includeStats {
		var successCount, errorCount int
		for _, t := range tiles {
			if t.Error != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		result["summary"] = map[string]interface{}{
			"total_tiles":   len(tiles),
			"success_tiles": successCount,
			"failed_tiles":  errorCount,
			"generated_at":  time.Now().UTC(),
		}
	}

	if f.pretty {
		return json.MarshalIndent(result, "", "  ")
	}
	return json.Marshal(result)
}

// ContentType returns the MIME type for JSON
func (f *JSONFormatter) ContentType() string {
	return "application/json"
}

// NewFormatter creates a formatter based on the specified configuration
func NewFormatter(config *FormatterConfig) (Formatter, error) {
	switch config.Format {
	case FormatGeoJSON:
		return NewGeoJSONFormatter(config.Pretty, config.IncludeStats), nil
	case FormatJSON:
		return NewJSONFormatter(config.Pretty, config.IncludeStats), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", config.Format)
	}
}

// FormatSingle is a convenience function to format a single tile
func FormatSingle(tile *tile.ProcessedTile, format Format, pretty bool) ([]byte, error) {
	config := &FormatterConfig{
		Format:       format,
		Pretty:       pretty,
		IncludeStats: false,
	}

	formatter, err := NewFormatter(config)
	if err != nil {
		return nil, err
	}

	return formatter.Format(tile)
}

// FormatBatch is a convenience function to format multiple tiles
func FormatBatch(tiles []*tile.ProcessedTile, format Format, pretty bool) ([]byte, error) {
	config := &FormatterConfig{
		Format:       format,
		Pretty:       pretty,
		IncludeStats: true,
	}

	formatter, err := NewFormatter(config)
	if err != nil {
		return nil, err
	}

	return formatter.FormatBatch(tiles)
}
