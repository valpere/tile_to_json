// pkg/mvt/converter_test.go - Unit tests for MVT converter
package mvt

import (
	"testing"

	"github.com/paulmach/orb"
)

func TestNewConverter(t *testing.T) {
	converter := NewConverter()
	if converter.options.CoordinateSystem != CoordSystemWebMercator {
		t.Errorf("Expected default coordinate system %s, got %s", CoordSystemWebMercator, converter.options.CoordinateSystem)
	}
}

func TestNewConverterWithOptions(t *testing.T) {
	options := &ConversionOptions{
		CoordinateSystem: CoordSystemWGS84,
		SimplifyGeometry: true,
	}
	
	converter, err := NewConverterWithOptions(options)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if converter.options.CoordinateSystem != CoordSystemWGS84 {
		t.Errorf("Expected coordinate system %s, got %s", CoordSystemWGS84, converter.options.CoordinateSystem)
	}
}

func TestValidateConversionOptions(t *testing.T) {
	tests := []struct {
		name    string
		options *ConversionOptions
		wantErr bool
	}{
		{
			name: "valid web-mercator",
			options: &ConversionOptions{
				CoordinateSystem: CoordSystemWebMercator,
			},
			wantErr: false,
		},
		{
			name: "valid wgs84",
			options: &ConversionOptions{
				CoordinateSystem: CoordSystemWGS84,
			},
			wantErr: false,
		},
		{
			name: "invalid coordinate system",
			options: &ConversionOptions{
				CoordinateSystem: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConversionOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConversionOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransformGeometryToWGS84(t *testing.T) {
	converter := NewConverter()
	
	// Test Web Mercator point (roughly New York City)
	webMercatorPoint := orb.Point{-8238310.24, 4969803.34}
	
	transformed := converter.transformGeometryToWGS84(webMercatorPoint)
	point := transformed.(orb.Point)
	
	// Should be approximately -74.006, 40.7128 (NYC coordinates)
	expectedLon := -74.006
	expectedLat := 40.7128
	tolerance := 0.1
	
	if abs(point[0]-expectedLon) > tolerance {
		t.Errorf("Longitude conversion incorrect: expected ~%f, got %f", expectedLon, point[0])
	}
	
	if abs(point[1]-expectedLat) > tolerance {
		t.Errorf("Latitude conversion incorrect: expected ~%f, got %f", expectedLat, point[1])
	}
}

func TestContains(t *testing.T) {
	converter := NewConverter()
	slice := []string{"water", "roads", "buildings"}
	
	if !converter.contains(slice, "water") {
		t.Error("Expected 'water' to be found in slice")
	}
	
	if converter.contains(slice, "parks") {
		t.Error("Expected 'parks' not to be found in slice")
	}
}

func TestConvertFeatureToGeoJSON(t *testing.T) {
	converter := NewConverter()
	
	feature := &DecodedFeature{
		ID:       "test-feature",
		Tags:     map[string]interface{}{"name": "Test", "type": "example"},
		Type:     "Point",
		Geometry: orb.Point{1.0, 2.0},
	}
	
	geoJSONFeature, err := converter.convertFeatureToGeoJSON(feature, "test-layer")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if geoJSONFeature.ID != "test-feature" {
		t.Errorf("Expected ID 'test-feature', got %v", geoJSONFeature.ID)
	}
	
	if geoJSONFeature.Properties["_layer"] != "test-layer" {
		t.Errorf("Expected layer 'test-layer', got %v", geoJSONFeature.Properties["_layer"])
	}
	
	if geoJSONFeature.Properties["name"] != "Test" {
		t.Errorf("Expected name 'Test', got %v", geoJSONFeature.Properties["name"])
	}
}

func TestPropertyFilter(t *testing.T) {
	options := &ConversionOptions{
		PropertyFilter:   []string{"name"},
		CoordinateSystem: CoordSystemWebMercator,
	}
	
	converter, _ := NewConverterWithOptions(options)
	
	feature := &DecodedFeature{
		Tags: map[string]interface{}{
			"name":        "Test",
			"type":        "example",
			"description": "Should be filtered out",
		},
		Geometry: orb.Point{1.0, 2.0},
	}
	
	geoJSONFeature, _ := converter.convertFeatureToGeoJSON(feature, "test-layer")
	
	if geoJSONFeature.Properties["name"] != "Test" {
		t.Error("Expected 'name' property to be included")
	}
	
	if _, exists := geoJSONFeature.Properties["description"]; exists {
		t.Error("Expected 'description' property to be filtered out")
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
