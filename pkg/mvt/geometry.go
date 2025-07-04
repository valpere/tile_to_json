// pkg/mvt/geometry.go - Shared geometry transformation utilities
package mvt

import "github.com/paulmach/orb"

// applyGeometryTransform applies a transformation function to all coordinates in a geometry
func applyGeometryTransform(geom orb.Geometry, transform func(orb.Point) orb.Point) orb.Geometry {
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
			result[i] = applyGeometryTransform(lineString, transform).(orb.LineString)
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
			result[i] = applyGeometryTransform(ring, transform).(orb.Ring)
		}
		return result
	case orb.MultiPolygon:
		result := make(orb.MultiPolygon, len(g))
		for i, polygon := range g {
			result[i] = applyGeometryTransform(polygon, transform).(orb.Polygon)
		}
		return result
	default:
		return geom
	}
}
