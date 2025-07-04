// pkg/mvt/decoder_test.go - Unit tests for MVT decoder
package mvt

import (
	"testing"

	"github.com/paulmach/orb"
)

func TestNewDecoder(t *testing.T) {
	decoder := NewDecoder()
	if decoder.extent != 4096 {
		t.Errorf("Expected default extent 4096, got %d", decoder.extent)
	}
}

func TestNewDecoderWithExtent(t *testing.T) {
	decoder := NewDecoderWithExtent(512)
	if decoder.extent != 512 {
		t.Errorf("Expected custom extent 512, got %d", decoder.extent)
	}
}

func TestDecode_EmptyData(t *testing.T) {
	decoder := NewDecoder()
	_, err := decoder.Decode([]byte{}, 1, 1, 1)
	if err == nil {
		t.Error("Expected error for empty data")
	}
	if err.Error() != "empty tile data" {
		t.Errorf("Expected 'empty tile data' error, got %s", err.Error())
	}
}

func TestTileIDString(t *testing.T) {
	tid := TileID{Z: 14, X: 8362, Y: 5956}
	expected := "14/8362/5956"
	if tid.String() != expected {
		t.Errorf("Expected %s, got %s", expected, tid.String())
	}
}

func TestTileIDValidate(t *testing.T) {
	tests := []struct {
		name    string
		tid     TileID
		wantErr bool
	}{
		{"valid coordinates", TileID{14, 8362, 5956}, false},
		{"invalid zoom negative", TileID{-1, 0, 0}, true},
		{"invalid zoom too high", TileID{23, 0, 0}, true},
		{"invalid x negative", TileID{1, -1, 0}, true},
		{"invalid x too high", TileID{1, 2, 0}, true},
		{"invalid y negative", TileID{1, 0, -1}, true},
		{"invalid y too high", TileID{1, 0, 2}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tid.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("TileID.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyGeometryTransform(t *testing.T) {
	// Simple identity transform for testing
	identityTransform := func(p orb.Point) orb.Point {
		return p
	}

	// Test Point
	point := orb.Point{1.0, 2.0}
	result := applyGeometryTransform(point, identityTransform)
	if result != point {
		t.Errorf("Expected %v, got %v", point, result)
	}

	// Test LineString
	lineString := orb.LineString{{1.0, 2.0}, {3.0, 4.0}}
	result = applyGeometryTransform(lineString, identityTransform)
	if len(result.(orb.LineString)) != 2 {
		t.Errorf("Expected LineString with 2 points, got %d", len(result.(orb.LineString)))
	}
}

func TestDecodedTileGetLayerNames(t *testing.T) {
	dt := &DecodedTile{
		Layers: map[string]*DecodedLayer{
			"water":  {},
			"roads":  {},
			"places": {},
		},
	}

	names := dt.GetLayerNames()
	expected := []string{"places", "roads", "water"} // Should be sorted
	
	if len(names) != len(expected) {
		t.Errorf("Expected %d layer names, got %d", len(expected), len(names))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected %s at position %d, got %s", expected[i], i, name)
		}
	}
}

func TestDecodedTileIsEmpty(t *testing.T) {
	// Empty tile
	emptyTile := &DecodedTile{
		Layers: map[string]*DecodedLayer{},
	}
	if !emptyTile.IsEmpty() {
		t.Error("Expected empty tile to return true for IsEmpty()")
	}

	// Non-empty tile
	nonEmptyTile := &DecodedTile{
		Layers: map[string]*DecodedLayer{
			"test": {
				Features: []*DecodedFeature{{}},
			},
		},
	}
	if nonEmptyTile.IsEmpty() {
		t.Error("Expected non-empty tile to return false for IsEmpty()")
	}
}
