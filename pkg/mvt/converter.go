// pkg/mvt/converter.go - MVT to GeoJSON conversion implementation
package mvt

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/paulmach/orb/geojson"
)

// Converter handles conversion of Mapbox Vector Tiles to GeoJSON format
type Converter struct {
	decoder *Decoder
	options *ConversionOptions
}

// ConversionOptions configures the conversion process
type ConversionOptions struct {
	IncludeMetadata bool     `json:"include_metadata"`
	LayerFilter     []string `json:"layer_filter,omitempty"`
	PropertyFilter  []string `json:"property_filter,omitempty"`
	SimplifyGeometry bool    `json:"simplify_geometry"`
	CoordinateSystem string  `json:"coordinate_system"` // "web-mercator" or "wgs84"
}

// ConversionMetadata contains metadata about the conversion process
type ConversionMetadata struct {
	Layers       []string `json:"layers"`
	FeatureCount int      `json:"feature_count"`
	Version      int      `json:"version"`
	Extent       int      `json:"extent"`
	TileID       string   `json:"tile_id"`
}

// NewConverter creates a new MVT to GeoJSON converter with default options
func NewConverter() *Converter {
	return &Converter{
		decoder: NewDecoder(),
		options: &ConversionOptions{
			IncludeMetadata:  false,
			SimplifyGeometry: false,
			CoordinateSystem: "web-mercator",
		},
	}
}

// NewConverterWithOptions creates a converter with custom options
func NewConverterWithOptions(options *ConversionOptions) *Converter {
	return &Converter{
		decoder: NewDecoder(),
		options: options,
	}
}

// Convert transforms MVT binary data to GeoJSON format
func (c *Converter) Convert(data []byte, z, x, y int) (map[string]interface{}, *ConversionMetadata, error) {
	// Decode the MVT data
	decodedTile, err := c.decoder.Decode(data, z, x, y)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode MVT: %w", err)
	}

	// Create GeoJSON FeatureCollection
	featureCollection := &geojson.FeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]*geojson.Feature, 0),
	}

	// Process each layer
	for layerName, layer := range decodedTile.Layers {
		// Apply layer filter if specified
		if len(c.options.LayerFilter) > 0 && !c.contains(c.options.LayerFilter, layerName) {
			continue
		}

		// Convert layer features to GeoJSON
		for _, feature := range layer.Features {
			geoJSONFeature, err := c.convertFeatureToGeoJSON(feature, layerName)
			if err != nil {
				// Log error but continue processing
				continue
			}

			featureCollection.Features = append(featureCollection.Features, geoJSONFeature)
		}
	}

	// Convert to coordinate system if specified
	if c.options.CoordinateSystem == "wgs84" {
		c.transformToWGS84(featureCollection)
	}

	// Create metadata
	metadata := &ConversionMetadata{
		Layers:       decodedTile.GetLayerNames(),
		FeatureCount: len(featureCollection.Features),
		Version:      decodedTile.Version,
		Extent:       decodedTile.Extent,
		TileID:       decodedTile.TileID.String(),
	}

	// Convert to map for JSON serialization
	result := map[string]interface{}{
		"type":     featureCollection.Type,
		"features": featureCollection.Features,
	}

	// Add metadata if requested
	if c.options.IncludeMetadata {
		result["metadata"] = metadata
	}

	return result, metadata, nil
}

// convertFeatureToGeoJSON converts a decoded feature to GeoJSON format
func (c *Converter) convertFeatureToGeoJSON(feature *DecodedFeature, layerName string) (*geojson.Feature, error) {
	// Create the GeoJSON feature
	geoJSONFeature := &geojson.Feature{
		Type:     "Feature",
		Geometry: feature.Geometry,
	}

	// Set feature ID if present
	if feature.ID != nil {
		geoJSONFeature.ID = *feature.ID
	}

	// Set properties
	properties := make(map[string]interface{})
	
	// Copy feature tags to properties
	for key, value := range feature.Tags {
		// Apply property filter if specified
		if len(c.options.PropertyFilter) > 0 && !c.contains(c.options.PropertyFilter, key) {
			continue
		}
		properties[key] = value
	}

	// Add layer name to properties
	properties["_layer"] = layerName

	geoJSONFeature.Properties = properties

	return geoJSONFeature, nil
}

// transformToWGS84 converts Web Mercator coordinates to WGS84 (longitude/latitude)
func (c *Converter) transformToWGS84(featureCollection *geojson.FeatureCollection) {
	const webMercatorMax = 20037508.342789244
	
	for _, feature := range featureCollection.Features {
		if feature.Geometry != nil {
			feature.Geometry = c.transformGeometryToWGS84(feature.Geometry)
		}
	}
}

// transformGeometryToWGS84 transforms a single geometry from Web Mercator to WGS84
func (c *Converter) transformGeometryToWGS84(geometry interface{}) interface{} {
	// This is a simplified transformation - in production, you might want to use
	// a proper projection library like proj4 for more accurate transformations
	const webMercatorMax = 20037508.342789244
	
	switch geom := geometry.(type) {
	case map[string]interface{}:
		if geomType, ok := geom["type"].(string); ok {
			switch geomType {
			case "Point":
				if coords, ok := geom["coordinates"].([]interface{}); ok && len(coords) == 2 {
					if x, ok := coords[0].(float64); ok {
						if y, ok := coords[1].(float64); ok {
							lon := (x / webMercatorMax) * 180.0
							lat := (2.0 * (Math.Atan(Math.Exp((y / webMercatorMax) * Math.Pi)) - Math.Pi / 4.0)) * 180.0 / Math.Pi
							geom["coordinates"] = []float64{lon, lat}
						}
					}
				}
			case "LineString", "MultiPoint":
				if coords, ok := geom["coordinates"].([]interface{}); ok {
					geom["coordinates"] = c.transformCoordinateArray(coords)
				}
			case "Polygon", "MultiLineString":
				if coords, ok := geom["coordinates"].([]interface{}); ok {
					for i, ring := range coords {
						if ringArray, ok := ring.([]interface{}); ok {
							coords[i] = c.transformCoordinateArray(ringArray)
						}
					}
				}
			case "MultiPolygon":
				if coords, ok := geom["coordinates"].([]interface{}); ok {
					for i, polygon := range coords {
						if polygonArray, ok := polygon.([]interface{}); ok {
							for j, ring := range polygonArray {
								if ringArray, ok := ring.([]interface{}); ok {
									polygonArray[j] = c.transformCoordinateArray(ringArray)
								}
							}
						}
					}
				}
			}
		}
	}
	
	return geometry
}

// transformCoordinateArray transforms an array of coordinates
func (c *Converter) transformCoordinateArray(coords []interface{}) []interface{} {
	const webMercatorMax = 20037508.342789244
	
	result := make([]interface{}, len(coords))
	for i, coord := range coords {
		if coordArray, ok := coord.([]interface{}); ok && len(coordArray) == 2 {
			if x, ok := coordArray[0].(float64); ok {
				if y, ok := coordArray[1].(float64); ok {
					lon := (x / webMercatorMax) * 180.0
					lat := (2.0 * (math.Atan(math.Exp((y / webMercatorMax) * math.Pi)) - math.Pi / 4.0)) * 180.0 / math.Pi
					result[i] = []float64{lon, lat}
					continue
				}
			}
		}
		result[i] = coord
	}
	return result
}

// contains checks if a slice contains a specific string
func (c *Converter) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ConvertToGeoJSONString converts MVT data to a GeoJSON string
func (c *Converter) ConvertToGeoJSONString(data []byte, z, x, y int, pretty bool) (string, error) {
	result, _, err := c.Convert(data, z, x, y)
	if err != nil {
		return "", err
	}

	var jsonData []byte
	if pretty {
		jsonData, err = json.MarshalIndent(result, "", "  ")
	} else {
		jsonData, err = json.Marshal(result)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to marshal GeoJSON: %w", err)
	}

	return string(jsonData), nil
}

// ValidateConversionOptions validates the conversion options
func ValidateConversionOptions(options *ConversionOptions) error {
	if options.CoordinateSystem != "web-mercator" && options.CoordinateSystem != "wgs84" {
		return fmt.Errorf("invalid coordinate system: %s, must be 'web-mercator' or 'wgs84'", options.CoordinateSystem)
	}
	return nil
}
