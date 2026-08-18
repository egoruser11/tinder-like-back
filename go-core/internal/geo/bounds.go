package geo

import (
	"errors"
	"math"
)

const (
	earthRadiusKM            = 6371.0088
	kilometersPerLatitudeDeg = 111.32
	DegreesToRadians         = math.Pi / 180
)

var ErrInvalidCoordinates = errors.New("invalid coordinates")
var ErrInvalidRadius = errors.New("invalid radius")

// Bounds is an axis-aligned geographic rectangle used as a cheap SQL
// pre-filter before calculating the exact distance.
type Bounds struct {
	MinLatitude  float64
	MaxLatitude  float64
	MinLongitude float64
	MaxLongitude float64
}

// NewBounds returns a bounding rectangle around a point. It is deliberately
// an approximation: callers must still check the exact distance afterwards.
func NewBounds(latitude, longitude, radiusKM float64) (Bounds, error) {
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return Bounds{}, ErrInvalidCoordinates
	}
	if radiusKM <= 0 {
		return Bounds{}, ErrInvalidRadius
	}

	latitudeDelta := radiusKM / kilometersPerLatitudeDeg
	latitudeRadians := latitude * DegreesToRadians
	longitudeKilometersPerDegree := kilometersPerLatitudeDeg * math.Cos(latitudeRadians)
	if math.Abs(longitudeKilometersPerDegree) < 1e-12 {
		return Bounds{
			MinLatitude:  math.Max(-90, latitude-latitudeDelta),
			MaxLatitude:  math.Min(90, latitude+latitudeDelta),
			MinLongitude: -180,
			MaxLongitude: 180,
		}, nil
	}

	longitudeDelta := radiusKM / longitudeKilometersPerDegree
	return Bounds{
		MinLatitude:  math.Max(-90, latitude-latitudeDelta),
		MaxLatitude:  math.Min(90, latitude+latitudeDelta),
		MinLongitude: math.Max(-180, longitude-longitudeDelta),
		MaxLongitude: math.Min(180, longitude+longitudeDelta),
	}, nil
}

// DistanceKM calculates the great-circle distance between two coordinates by
// the Haversine formula.
func DistanceKM(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	lat1 := latitude1 * DegreesToRadians
	lat2 := latitude2 * DegreesToRadians
	deltaLat := (latitude2 - latitude1) * DegreesToRadians
	deltaLon := (longitude2 - longitude1) * DegreesToRadians

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
