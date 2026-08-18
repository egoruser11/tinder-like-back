package geo

import (
	"errors"
	"math"
	"testing"
)

func TestNewBounds_MoscowRadius100KM(t *testing.T) {
	bounds, err := NewBounds(55.7558, 37.6173, 100)
	if err != nil {
		t.Fatalf("NewBounds returned error: %v", err)
	}

	assertClose(t, bounds.MinLatitude, 54.8575, 0.001)
	assertClose(t, bounds.MaxLatitude, 56.6541, 0.001)
	assertClose(t, bounds.MinLongitude, 36.02, 0.01)
	assertClose(t, bounds.MaxLongitude, 39.21, 0.01)
}

func TestNewBounds_RejectsInvalidInput(t *testing.T) {
	_, err := NewBounds(91, 37, 100)
	if !errors.Is(err, ErrInvalidCoordinates) {
		t.Fatalf("error = %v, want ErrInvalidCoordinates", err)
	}

	_, err = NewBounds(55, 37, 0)
	if !errors.Is(err, ErrInvalidRadius) {
		t.Fatalf("error = %v, want ErrInvalidRadius", err)
	}
}

func TestDistanceKM(t *testing.T) {
	assertClose(t, DistanceKM(55.7558, 37.6173, 55.7558, 37.6173), 0, 0.0001)

	// One degree of latitude near Moscow is about 111.2 km.
	assertClose(t, DistanceKM(55.7558, 37.6173, 56.7558, 37.6173), 111.2, 0.5)
}

func assertClose(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.6f, want %.6f ± %.6f", got, want, tolerance)
	}
}
