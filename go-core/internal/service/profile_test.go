package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinder-core/internal/models"
)

func TestProfileServiceSave_NormalizesAndSavesProfile(t *testing.T) {
	store := &fakeProfileRepository{}
	service := NewProfileService(store)
	service.now = func() time.Time { return time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC) }
	city := " Москва "
	latitude, longitude := 55.7558, 37.6173

	_, err := service.Save(context.Background(), 7, SaveProfileInput{
		FullName:  "  Иван  ",
		Birthday:  "2000-01-02",
		Gender:    1,
		Bio:       "  Привет  ",
		Latitude:  &latitude,
		Longitude: &longitude,
		City:      &city,
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if store.saved.UserID != 7 || store.saved.FullName != "Иван" || store.saved.Bio != "Привет" || store.saved.City == nil || *store.saved.City != "Москва" {
		t.Fatalf("profile was not normalized: %#v", store.saved)
	}
}

func TestProfileServiceSave_RejectsInvalidCoordinates(t *testing.T) {
	service := NewProfileService(&fakeProfileRepository{})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC) }
	latitude := 55.0

	_, err := service.Save(context.Background(), 7, SaveProfileInput{
		FullName: "Иван", Birthday: "2000-01-02", Gender: 1, Latitude: &latitude,
	})
	if !errors.Is(err, ErrCoordinatesMustBeTogether) {
		t.Fatalf("error = %v, want ErrCoordinatesMustBeTogether", err)
	}
}

func TestProfileServiceSave_RejectsUnderageOwner(t *testing.T) {
	service := NewProfileService(&fakeProfileRepository{})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC) }

	_, err := service.Save(context.Background(), 7, SaveProfileInput{
		FullName: "Иван", Birthday: "2010-01-02", Gender: 1,
	})
	if !errors.Is(err, ErrProfileOwnerMustBeAdult) {
		t.Fatalf("error = %v, want ErrProfileOwnerMustBeAdult", err)
	}
}

type fakeProfileRepository struct {
	saved models.Profile
}

func (r *fakeProfileRepository) GetByUserID(context.Context, int64) (*models.Profile, error) {
	return nil, nil
}

func (r *fakeProfileRepository) Upsert(_ context.Context, profile models.Profile) (*models.Profile, error) {
	r.saved = profile
	return &profile, nil
}

func (r *fakeProfileRepository) Deactivate(context.Context, int64) error { return nil }
