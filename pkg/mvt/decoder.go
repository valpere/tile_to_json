// pkg/mvt/decoder.go - Mapbox Vector Tile decoding implementation
package mvt

import (
	"fmt"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
)

// Decoder handles decoding of Mapbox Vector Tiles from Protocol Buffer format
type Decoder struct {
	extent int
}

// NewDecoder creates a new MVT decoder with default settings
func NewDecoder() *Decoder {
	return &Decoder{
		extent: 4096, // Default MVT extent
	}
}

// NewDecoderWithExtent creates a new MVT decoder with custom extent
func NewDecoderWithExtent(extent int) *Decoder {
	return &Decoder{
		extent: extent,
	}
}

// DecodedTile represents a decoded MVT tile with its layers and metadata
type DecodedTile struct {
	Layers  map[string]*DecodedLayer `json:"layers"`
	Extent  int                      `json:"extent"`
	Version int                      `json:"version"`
	TileID  TileID                   `json:"tile_id"`
}

// DecodedLayer represents a single layer within an MVT tile
type DecodedLayer struct {
	Name     string            `json:"name"`
	Features []*DecodedFeature `json:"features"`
	Extent   int               `json:"extent"`
	Version  int               `json:"version"`
	Keys     []string          `json:"keys,omitempty"`
	Values   []interface{}     `json:"values,omitempty"`
}

// DecodedFeature represents a single feature within a layer
type DecodedFeature struct {
	ID       *uint64                `json:"id,omitempty"`
	Tags     map[string]interface{} `json:"tags"`
	Type     geojson.GeometryType   `json:"type"`
	Geometry orb.Geometry           `json:"geometry"`
}

// TileID represents the tile coordinates and zoom level
type TileID struct {
	Z int `json:"z"`
	X int `json:"x"`
	Y int `json:"y"`
}

// Decode decodes a Mapbox Vector Tile from binary Protocol Buffer data
func (d *Decoder) Decode(data []byte, z, x, y int) (*DecodedTile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty tile data")
	}

	// Parse the MVT data using the orb library
	layers, err := mvt.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal MVT data: %w", err)
	}

	// Create the decoded tile structure
	decodedTile := &DecodedTile{
		Layers:  make(map[string]*DecodedLayer),
		Extent:  d.extent,
		Version: 2, // MVT specification version
		TileID: TileID{
			Z: z,
			X: x,
			Y: y,
		},
	}

	// Process each layer
	for layerName, layer := range layers {
		decodedLayer, err := d.decodeLayer(layerName, layer, z, x, y)
		if err != nil {
			return nil, fmt.Errorf("failed to decode layer %s: %w", layerName, err)
		}
		decodedTile.Layers[layerName] = decodedLayer
	}

	return decodedTile, nil
}

// decodeLayer processes a single layer from the MVT data
func (d *Decoder) decodeLayer(layerName string, layer *mvt.Layer, z, x, y int) (*DecodedLayer, error) {
	decodedLayer := &DecodedLayer{
		Name:     layerName,
		Features: make([]*DecodedFeature, 0, len(layer.Features)),
		Extent:   int(layer.Extent),
		Version:  int(layer.Version),
	}

	// Process each feature in the layer
	for _, feature := range layer.Features {
		decodedFeature, err := d.decodeFeature(feature, z, x, y)
		if err != nil {
			// Log the error but continue processing other features
			continue
		}
		decodedLayer.Features = append(decodedLayer.Features, decodedFeature)
	}

	return decodedLayer, nil
}

// decodeFeature processes a single feature from the layer
func (d *Decoder) decodeFeature(feature *mvt.Feature, z, x, y int) (*DecodedFeature, error) {
	// Convert the feature geometry to geographic coordinates
	geometry := feature.Geometry
	if geometry == nil {
		return nil, fmt.Errorf("feature has no geometry")
	}

	// Transform tile coordinates to geographic coordinates
	transformedGeometry := d.transformGeometry(geometry, z, x, y)

	decodedFeature := &DecodedFeature{
		Tags:     feature.Tags,
		Geometry: transformedGeometry,
	}

	// Set feature ID if present
	if feature.ID != nil {
		decodedFeature.ID = feature.ID
	}

	// Determine geometry type
	switch transformedGeometry.(type) {
	case orb.Point:
		decodedFeature.Type = geojson.TypePoint
	case orb.MultiPoint:
		decodedFeature.Type = geojson.TypeMultiPoint
	case orb.LineString:
		decodedFeature.Type = geojson.TypeLineString
	case orb.MultiLineString:
		decodedFeature.Type = geojson.TypeMultiLineString
	case orb.Polygon:
		decodedFeature.Type = geojson.TypePolygon
	case orb.MultiPolygon:
		decodedFeature.Type = geojson.TypeMultiPolygon
	default:
		return nil, fmt.Errorf("unsupported geometry type: %T", transformedGeometry)
	}

	return decodedFeature, nil
}

// transformGeometry converts tile coordinates to geographic coordinates (Web Mercator)
func (d *Decoder) transformGeometry(geometry orb.Geometry, z, x, y int) orb.Geometry {
	// Calculate the transformation parameters
	n := float64(1 << uint(z))
	tileSize := float64(d.extent)

	// Web Mercator bounds
	const webMercatorMax = 20037508.342789244

	transform := func(point orb.Point) orb.Point {
		// Convert tile pixel coordinates to tile fractional coordinates
		tileX := point[0] / tileSize
		tileY := point[1] / tileSize

		// Convert to global tile coordinates
		globalX := (float64(x) + tileX) / n
		globalY := (float64(y) + tileY) / n

		// Convert to Web Mercator coordinates
		mercatorX := (globalX*2.0 - 1.0) * webMercatorMax
		mercatorY := (1.0 - globalY*2.0) * webMercatorMax

		return orb.Point{mercatorX, mercatorY}
	}

	return orb.Transform(geometry, transform)
}

// GetLayerNames returns the names of all layers in the decoded tile
func (dt *DecodedTile) GetLayerNames() []string {
	names := make([]string, 0, len(dt.Layers))
	for name := range dt.Layers {
		names = append(names, name)
	}
	return names
}

// GetFeatureCount returns the total number of features across all layers
func (dt *DecodedTile) GetFeatureCount() int {
	count := 0
	for _, layer := range dt.Layers {
		count += len(layer.Features)
	}
	return count
}

// GetLayerFeatureCount returns the number of features in a specific layer
func (dt *DecodedTile) GetLayerFeatureCount(layerName string) int {
	if layer, exists := dt.Layers[layerName]; exists {
		return len(layer.Features)
	}
	return 0
}

// HasLayer checks if the tile contains a specific layer
func (dt *DecodedTile) HasLayer(layerName string) bool {
	_, exists := dt.Layers[layerName]
	return exists
}

// IsEmpty returns true if the tile contains no features
func (dt *DecodedTile) IsEmpty() bool {
	return dt.GetFeatureCount() == 0
}

// String returns a string representation of the tile ID
func (tid TileID) String() string {
	return fmt.Sprintf("%d/%d/%d", tid.Z, tid.X, tid.Y)
}

// Validate checks if the tile coordinates are valid
func (tid TileID) Validate() error {
	if tid.Z < 0 || tid.Z > 22 {
		return fmt.Errorf("invalid zoom level %d: must be between 0 and 22", tid.Z)
	}

	maxTile := 1 << uint(tid.Z)
	if tid.X < 0 || tid.X >= maxTile {
		return fmt.Errorf("invalid X coordinate %d for zoom %d: must be between 0 and %d", tid.X, tid.Z, maxTile-1)
	}

	if tid.Y < 0 || tid.Y >= maxTile {
		return fmt.Errorf("invalid Y coordinate %d for zoom %d: must be between 0 and %d", tid.Y, tid.Z, maxTile-1)
	}

	return nil
}
