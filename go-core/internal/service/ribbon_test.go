package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinder-core/internal/events"
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
	service := NewRibbonService(store, profiles, nil, nil)

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
		nil,
	)

	_, err := service.GetFeed(context.Background(), 1, FeedInput{Limit: 1})
	if !errors.Is(err, ErrProfileCoordinatesRequired) {
		t.Fatalf("error = %v, want ErrProfileCoordinatesRequired", err)
	}
}

func TestGetFeed_RejectsInvalidCursor(t *testing.T) {
	service := NewRibbonService(&fakeRibbonStore{}, &fakeProfileStore{}, nil, nil)
	_, err := service.GetFeed(context.Background(), 1, FeedInput{Limit: 1, Cursor: "not-an-id"})
	if !errors.Is(err, ErrInvalidFeedCursor) {
		t.Fatalf("error = %v, want ErrInvalidFeedCursor", err)
	}
}

func TestLike_ReturnsMatchWhenRepositoryCreatesChat(t *testing.T) {
	chatID := int64(42)
	store := &fakeRibbonStore{likeChatID: &chatID}
	service := NewRibbonService(store, &fakeProfileStore{}, nil, nil)

	output, err := service.Like(context.Background(), 1, TargetInput{TargetUserID: 2})
	if err != nil {
		t.Fatalf("Like returned error: %v", err)
	}
	if !output.Matched || output.ChatID == nil || *output.ChatID != chatID {
		t.Fatalf("unexpected like output: %#v", output)
	}
	if store.likedActorID != 1 || store.likedTargetID != 2 {
		t.Fatalf("repository called with %d -> %d, want 1 -> 2", store.likedActorID, store.likedTargetID)
	}
}

func TestLike_PublishesSwipeAndMatchEvents(t *testing.T) {
	chatID := int64(42)
	store := &fakeRibbonStore{likeChatID: &chatID}
	publisher := &fakeEventPublisher{}
	service := NewRibbonService(store, &fakeProfileStore{}, publisher, nil)

	if _, err := service.Like(context.Background(), 1, TargetInput{TargetUserID: 2}); err != nil {
		t.Fatalf("Like returned error: %v", err)
	}
	if len(publisher.events) != 2 || publisher.events[0].Type != "swipe" || publisher.events[1].Type != "match" {
		t.Fatalf("unexpected published events: %#v", publisher.events)
	}
	if publisher.events[0].Payload["user_id"] != int64(1) || publisher.events[0].Payload["target_user_id"] != int64(2) {
		t.Fatalf("unexpected swipe payload: %#v", publisher.events[0].Payload)
	}
}

func TestRibbonActions_RejectSelfTargetBeforeRepository(t *testing.T) {
	store := &fakeRibbonStore{}
	service := NewRibbonService(store, &fakeProfileStore{}, nil, nil)

	_, err := service.Like(context.Background(), 1, TargetInput{TargetUserID: 1})
	if !errors.Is(err, ErrInvalidTargetUser) {
		t.Fatalf("error = %v, want ErrInvalidTargetUser", err)
	}
	if store.likedTargetID != 0 {
		t.Fatal("repository must not be called for an invalid target")
	}
}

func TestReport_ValidatesAndTrimsComment(t *testing.T) {
	store := &fakeRibbonStore{}
	service := NewRibbonService(store, &fakeProfileStore{}, nil, nil)

	err := service.Report(context.Background(), 1, ReportInput{
		TargetUserID: 2,
		Reason:       2,
		Comment:      "  spam links  ",
	})
	if err != nil {
		t.Fatalf("Report returned error: %v", err)
	}
	if store.reportReason != 2 || store.reportComment == nil || *store.reportComment != "spam links" {
		t.Fatalf("unexpected report captured by repository: reason=%d comment=%v", store.reportReason, store.reportComment)
	}

	err = service.Report(context.Background(), 1, ReportInput{TargetUserID: 2, Reason: 6})
	if !errors.Is(err, ErrInvalidReportReason) {
		t.Fatalf("error = %v, want ErrInvalidReportReason", err)
	}
}

func TestSavePreferences_NormalizesOptionalCity(t *testing.T) {
	store := &fakeRibbonStore{}
	service := NewRibbonService(store, &fakeProfileStore{}, nil, nil)
	city := "  Москва "
	gender := int16(2)

	_, err := service.SavePreferences(context.Background(), 7, SavePreferencesInput{
		City:          &city,
		MinAge:        20,
		MaxAge:        35,
		Gender:        &gender,
		IsVerified:    true,
		MaxDistanceKM: 70,
	})
	if err != nil {
		t.Fatalf("SavePreferences returned error: %v", err)
	}
	if store.savedPreferences.UserID != 7 || store.savedPreferences.City == nil || *store.savedPreferences.City != "Москва" {
		t.Fatalf("unexpected saved preferences: %#v", store.savedPreferences)
	}
}

func TestSavePreferences_RejectsInvalidRange(t *testing.T) {
	service := NewRibbonService(&fakeRibbonStore{}, &fakeProfileStore{}, nil, nil)
	_, err := service.SavePreferences(context.Background(), 7, SavePreferencesInput{
		MinAge: 30, MaxAge: 20, MaxDistanceKM: 10,
	})
	if !errors.Is(err, ErrInvalidDiscoveryAge) {
		t.Fatalf("error = %v, want ErrInvalidDiscoveryAge", err)
	}
}

type fakeRibbonStore struct {
	filters          *models.DiscoveryPreferences
	filtersErr       error
	candidateBatches [][]models.DiscoveryCandidate
	queries          []repository.CandidateQuery
	incomingLikes    []models.DiscoveryCandidate
	likeChatID       *int64
	likedActorID     int64
	likedTargetID    int64
	reportReason     int16
	reportComment    *string
	savedPreferences models.DiscoveryPreferences
}

func (s *fakeRibbonStore) GetFilters(context.Context, int64) (*models.DiscoveryPreferences, error) {
	return s.filters, s.filtersErr
}

func (s *fakeRibbonStore) UpsertFilters(_ context.Context, preferences models.DiscoveryPreferences) (*models.DiscoveryPreferences, error) {
	s.savedPreferences = preferences
	return &preferences, nil
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

func (s *fakeRibbonStore) ListIncomingLikes(context.Context, int64, int64, int) ([]models.DiscoveryCandidate, error) {
	return s.incomingLikes, nil
}

func (s *fakeRibbonStore) CreateLike(_ context.Context, actorUserID, targetUserID int64) (*int64, error) {
	s.likedActorID = actorUserID
	s.likedTargetID = targetUserID
	return s.likeChatID, nil
}

func (s *fakeRibbonStore) CreateDislike(context.Context, int64, int64) error { return nil }

func (s *fakeRibbonStore) CreateBlock(context.Context, int64, int64) error { return nil }

func (s *fakeRibbonStore) RemoveBlock(context.Context, int64, int64) error { return nil }

func (s *fakeRibbonStore) CreateReport(_ context.Context, _ int64, _ int64, reason int16, comment *string) error {
	s.reportReason = reason
	s.reportComment = comment
	return nil
}

type fakeProfileStore struct {
	profile *models.Profile
	err     error
}

func (s *fakeProfileStore) GetByUserID(context.Context, int64) (*models.Profile, error) {
	return s.profile, s.err
}

type fakeEventPublisher struct {
	events []events.Event
}

func (p *fakeEventPublisher) Publish(event events.Event) {
	p.events = append(p.events, event)
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
