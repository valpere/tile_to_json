// pkg/mvt/decoder.go - Mapbox Vector Tile decoding implementation
package mvt

import (
	"fmt"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
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
}

// DecodedFeature represents a single feature within a layer
type DecodedFeature struct {
	ID       interface{}            `json:"id,omitempty"`
	Tags     map[string]interface{} `json:"tags"`
	Type     string                 `json:"type"`
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

	// Use orb library to unmarshal MVT data
	layers, err := mvt.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal MVT data: %w", err)
	}

	// Create the decoded tile structure
	decodedTile := &DecodedTile{
		Layers:  make(map[string]*DecodedLayer),
		Extent:  d.extent,
		Version: 2,
		TileID: TileID{
			Z: z,
			X: x,
			Y: y,
		},
	}

	// Process each layer - layers is mvt.Layers which can be ranged over
	for _, layer := range layers {
		decodedLayer := &DecodedLayer{
			Name:     layer.Name,
			Features: make([]*DecodedFeature, 0, len(layer.Features)),
			Extent:   int(layer.Extent),
			Version:  int(layer.Version),
		}

		// Process each feature - layer.Features is []*geojson.Feature
		for _, feature := range layer.Features {
			decodedFeature := &DecodedFeature{
				ID:       feature.ID,
				Tags:     feature.Properties,
				Geometry: d.transformGeometry(feature.Geometry, z, x, y),
			}

			// Determine geometry type
			switch feature.Geometry.(type) {
			case orb.Point:
				decodedFeature.Type = "Point"
			case orb.MultiPoint:
				decodedFeature.Type = "MultiPoint"
			case orb.LineString:
				decodedFeature.Type = "LineString"
			case orb.MultiLineString:
				decodedFeature.Type = "MultiLineString"
			case orb.Polygon:
				decodedFeature.Type = "Polygon"
			case orb.MultiPolygon:
				decodedFeature.Type = "MultiPolygon"
			default:
				decodedFeature.Type = "Unknown"
			}

			decodedLayer.Features = append(decodedLayer.Features, decodedFeature)
		}

		decodedTile.Layers[layer.Name] = decodedLayer
	}

	return decodedTile, nil
}

// transformGeometry converts tile coordinates to geographic coordinates
func (d *Decoder) transformGeometry(geometry orb.Geometry, z, x, y int) orb.Geometry {
	numTiles := 1 << uint(z)
	n := float64(numTiles)
	tileSize := float64(d.extent)
	const webMercatorMax = 20037508.342789244

	transform := func(point orb.Point) orb.Point {
		tileX := point[0] / tileSize
		tileY := point[1] / tileSize
		globalX := (float64(x) + tileX) / n
		globalY := (float64(y) + tileY) / n
		mercatorX := (globalX*2.0 - 1.0) * webMercatorMax
		mercatorY := (1.0 - globalY*2.0) * webMercatorMax
		return orb.Point{mercatorX, mercatorY}
	}

	return transformGeometry(geometry, transform)
}

// transformGeometry applies transformation to geometry
func transformGeometry(geom orb.Geometry, transform func(orb.Point) orb.Point) orb.Geometry {
	switch g := geom.(type) {
	case orb.Point:
		return transform(g)
	case orb.MultiPoint:
		result := make(orb.MultiPoint, len(g))
		for i, point := range g {
			result[i] = transform(point)
		}
		return result
	case orb.LineString:
		result := make(orb.LineString, len(g))
		for i, point := range g {
			result[i] = transform(point)
		}
		return result
	case orb.MultiLineString:
		result := make(orb.MultiLineString, len(g))
		for i, lineString := range g {
			result[i] = transformGeometry(lineString, transform).(orb.LineString)
		}
		return result
	case orb.Ring:
		result := make(orb.Ring, len(g))
		for i, point := range g {
			result[i] = transform(point)
		}
		return result
	case orb.Polygon:
		result := make(orb.Polygon, len(g))
		for i, ring := range g {
			result[i] = transformGeometry(ring, transform).(orb.Ring)
		}
		return result
	case orb.MultiPolygon:
		result := make(orb.MultiPolygon, len(g))
		for i, polygon := range g {
			result[i] = transformGeometry(polygon, transform).(orb.Polygon)
		}
		return result
	default:
		return geom
	}
}

// GetLayerNames returns layer names
func (dt *DecodedTile) GetLayerNames() []string {
	names := make([]string, 0, len(dt.Layers))
	for name := range dt.Layers {
		names = append(names, name)
	}
	return names
}

// GetFeatureCount returns total feature count
func (dt *DecodedTile) GetFeatureCount() int {
	count := 0
	for _, layer := range dt.Layers {
		count += len(layer.Features)
	}
	return count
}

// GetLayerFeatureCount returns feature count for specific layer
func (dt *DecodedTile) GetLayerFeatureCount(layerName string) int {
	if layer, exists := dt.Layers[layerName]; exists {
		return len(layer.Features)
	}
	return 0
}

// HasLayer checks if layer exists
func (dt *DecodedTile) HasLayer(layerName string) bool {
	_, exists := dt.Layers[layerName]
	return exists
}

// IsEmpty checks if tile has no features
func (dt *DecodedTile) IsEmpty() bool {
	return dt.GetFeatureCount() == 0
}

// String returns string representation of tile ID
func (tid TileID) String() string {
	return fmt.Sprintf("%d/%d/%d", tid.Z, tid.X, tid.Y)
}

// Validate checks tile coordinate validity
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
