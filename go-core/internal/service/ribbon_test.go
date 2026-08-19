package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinder-core/internal/models"
	"tinder-core/internal/repository"
)

func TestGetFeed_FiltersExactDistanceAfterSQLBounds(t *testing.T) {
	viewerLat, viewerLon := 55.7558, 37.6173
	nearLat, nearLon := 55.8058, 37.6173
	farLat, farLon := 56.6558, 37.6173
	store := &fakeRibbonStore{
		filters: &models.DiscoveryPreferences{MinAge: 18, MaxAge: 99, MaxDistanceKM: 20},
		candidateBatches: [][]models.DiscoveryCandidate{{
			candidate(2, 2, nearLat, nearLon),
			candidate(3, 3, farLat, farLon),
		}},
	}
	profiles := &fakeProfileStore{profile: &models.Profile{Latitude: &viewerLat, Longitude: &viewerLon}}
	service := NewRibbonService(store, profiles, nil)

	output, err := service.GetFeed(context.Background(), 1, FeedInput{Limit: 2})
	if err != nil {
		t.Fatalf("GetFeed returned error: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ProfileID != 2 {
		t.Fatalf("unexpected feed items: %#v", output.Items)
	}
	if output.Items[0].DistanceKM <= 0 || output.Items[0].DistanceKM >= 20 {
		t.Fatalf("distance = %v, want 0 < distance < 20", output.Items[0].DistanceKM)
	}
	if len(store.queries) != 1 || store.queries[0].Limit != 6 {
		t.Fatalf("unexpected candidate queries: %#v", store.queries)
	}
	if store.queries[0].Bounds.MinLongitude >= viewerLon || store.queries[0].Bounds.MaxLongitude <= viewerLon {
		t.Fatalf("bounds do not surround viewer longitude: %#v", store.queries[0].Bounds)
	}
}

func TestGetFeed_RequiresProfileCoordinates(t *testing.T) {
	service := NewRibbonService(
		&fakeRibbonStore{filters: &models.DiscoveryPreferences{MaxDistanceKM: 20}},
		&fakeProfileStore{profile: &models.Profile{}},
		nil,
	)

	_, err := service.GetFeed(context.Background(), 1, FeedInput{Limit: 1})
	if !errors.Is(err, ErrProfileCoordinatesRequired) {
		t.Fatalf("error = %v, want ErrProfileCoordinatesRequired", err)
	}
}

func TestGetFeed_RejectsInvalidCursor(t *testing.T) {
	service := NewRibbonService(&fakeRibbonStore{}, &fakeProfileStore{}, nil)
	_, err := service.GetFeed(context.Background(), 1, FeedInput{Limit: 1, Cursor: "not-an-id"})
	if !errors.Is(err, ErrInvalidFeedCursor) {
		t.Fatalf("error = %v, want ErrInvalidFeedCursor", err)
	}
}

type fakeRibbonStore struct {
	filters          *models.DiscoveryPreferences
	filtersErr       error
	candidateBatches [][]models.DiscoveryCandidate
	queries          []repository.CandidateQuery
}

func (s *fakeRibbonStore) GetFilters(context.Context, int64) (*models.DiscoveryPreferences, error) {
	return s.filters, s.filtersErr
}

func (s *fakeRibbonStore) ListCandidates(_ context.Context, query repository.CandidateQuery) ([]models.DiscoveryCandidate, error) {
	s.queries = append(s.queries, query)
	if len(s.candidateBatches) == 0 {
		return nil, nil
	}
	batch := s.candidateBatches[0]
	s.candidateBatches = s.candidateBatches[1:]
	return batch, nil
}

type fakeProfileStore struct {
	profile *models.Profile
	err     error
}

func (s *fakeProfileStore) GetByUserID(context.Context, int64) (*models.Profile, error) {
	return s.profile, s.err
}

func candidate(profileID, userID int64, latitude, longitude float64) models.DiscoveryCandidate {
	return models.DiscoveryCandidate{
		Profile: models.Profile{
			ID:        profileID,
			UserID:    userID,
			FullName:  "Candidate",
			Birthday:  time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
			Gender:    2,
			Latitude:  &latitude,
			Longitude: &longitude,
		},
		IsVerified: true,
	}
}
