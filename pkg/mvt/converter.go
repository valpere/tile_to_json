// pkg/mvt/converter.go - MVT to GeoJSON conversion implementation
package mvt

import (
	"encoding/json"
	"fmt"
	"log"
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/simplify"
)

// Converter handles conversion of Mapbox Vector Tiles to GeoJSON format
type Converter struct {
	decoder *Decoder
	options *ConversionOptions
}

// ConversionOptions configures the conversion process
type ConversionOptions struct {
	IncludeMetadata  bool     `json:"include_metadata"`  // Include tile metadata in output
	LayerFilter      []string `json:"layer_filter,omitempty"`      // Only include specified layers
	PropertyFilter   []string `json:"property_filter,omitempty"`   // Only include specified properties
	SimplifyGeometry bool     `json:"simplify_geometry"`           // Simplify geometries using Douglas-Peucker
	CoordinateSystem string   `json:"coordinate_system"`           // "web-mercator" or "wgs84"
}

// ConversionMetadata contains metadata about the conversion process
type ConversionMetadata struct {
	Layers       []string `json:"layers"`
	FeatureCount int      `json:"feature_count"`
	Version      int      `json:"version"`
	Extent       int      `json:"extent"`
	TileID       string   `json:"tile_id"`
}

// Coordinate system constants
const (
	CoordSystemWebMercator = "web-mercator"
	CoordSystemWGS84       = "wgs84"
)

// NewConverter creates a new MVT to GeoJSON converter with default options
func NewConverter() *Converter {
	options := &ConversionOptions{
		IncludeMetadata:  false,
		SimplifyGeometry: false,
		CoordinateSystem: CoordSystemWebMercator,
	}
	
	if err := ValidateConversionOptions(options); err != nil {
		log.Printf("Warning: invalid default options: %v", err)
	}

	return &Converter{
		decoder: NewDecoder(),
		options: options,
	}
}

// NewConverterWithOptions creates a converter with custom options
func NewConverterWithOptions(options *ConversionOptions) (*Converter, error) {
	if err := ValidateConversionOptions(options); err != nil {
		return nil, fmt.Errorf("invalid conversion options: %w", err)
	}

	return &Converter{
		decoder: NewDecoder(),
		options: options,
	}, nil
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

	var conversionErrors []error

	// Process each layer
	for layerName, layer := range decodedTile.Layers {
		// Apply layer filter if specified
		if len(c.options.LayerFilter) > 0 && !c.contains(c.options.LayerFilter, layerName) {
			continue
		}

		// Convert layer features to GeoJSON
		for _, feature := range layer.Features {
			// Skip features with nil geometry
			if feature.Geometry == nil {
				log.Printf("Warning: skipping feature with nil geometry in layer %s", layerName)
				continue
			}

			geoJSONFeature, err := c.convertFeatureToGeoJSON(feature, layerName)
			if err != nil {
				conversionErrors = append(conversionErrors, fmt.Errorf("layer %s: %w", layerName, err))
				continue
			}

			// Apply geometry simplification if enabled
			if c.options.SimplifyGeometry && geoJSONFeature.Geometry != nil {
				geoJSONFeature.Geometry = simplify.DouglasPeucker(1.0).Simplify(geoJSONFeature.Geometry)
			}

			featureCollection.Features = append(featureCollection.Features, geoJSONFeature)
		}
	}

	// Log conversion errors for debugging
	if len(conversionErrors) > 0 {
		log.Printf("Conversion completed with %d errors", len(conversionErrors))
		for _, err := range conversionErrors {
			log.Printf("Conversion error: %v", err)
		}
	}

	// Convert to coordinate system if specified
	if c.options.CoordinateSystem == CoordSystemWGS84 {
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
		geoJSONFeature.ID = feature.ID
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
	for _, feature := range featureCollection.Features {
		if feature.Geometry != nil {
			feature.Geometry = c.transformGeometryToWGS84(feature.Geometry)
		}
	}
}

// transformGeometryToWGS84 transforms a single geometry from Web Mercator to WGS84
func (c *Converter) transformGeometryToWGS84(geometry orb.Geometry) orb.Geometry {
	const webMercatorMax = 20037508.342789244

	transform := func(point orb.Point) orb.Point {
		x, y := point[0], point[1]
		
		// Convert Web Mercator to WGS84 using correct formulas
		lon := (x / webMercatorMax) * 180.0
		
		// Correct Web Mercator to latitude conversion
		lat := y / webMercatorMax
		lat = 180.0/math.Pi * (2*math.Atan(math.Exp(lat*math.Pi)) - math.Pi/2.0)
		
		return orb.Point{lon, lat}
	}

	return applyGeometryTransform(geometry, transform)
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
	if options.CoordinateSystem != CoordSystemWebMercator && options.CoordinateSystem != CoordSystemWGS84 {
		return fmt.Errorf("invalid coordinate system: %s, must be '%s' or '%s'", 
			options.CoordinateSystem, CoordSystemWebMercator, CoordSystemWGS84)
	}
	return nil
}
