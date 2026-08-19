package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"tinder-core/internal/models"
)

var (
	ErrProfileNameRequired       = errors.New("profile name is required")
	ErrInvalidProfileBirthday    = errors.New("invalid profile birthday")
	ErrProfileOwnerMustBeAdult   = errors.New("profile owner must be at least 18")
	ErrInvalidProfileGender      = errors.New("invalid profile gender")
	ErrCoordinatesMustBeTogether = errors.New("latitude and longitude must be supplied together")
	ErrInvalidProfileCoordinates = errors.New("invalid profile coordinates")
)

type profileStore interface {
	GetByUserID(context.Context, int64) (*models.Profile, error)
	Upsert(context.Context, models.Profile) (*models.Profile, error)
	Deactivate(context.Context, int64) error
}

// ProfileService owns validation and lifecycle rules for a user's profile.
type ProfileService struct {
	repository profileStore
	now        func() time.Time
}

func NewProfileService(repository profileStore) *ProfileService {
	return &ProfileService{repository: repository, now: time.Now}
}

type SaveProfileInput struct {
	FullName  string   `json:"full_name"`
	Birthday  string   `json:"birthday"`
	Gender    int16    `json:"gender"`
	Bio       string   `json:"bio"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	City      *string  `json:"city"`
}

func (s *ProfileService) Get(ctx context.Context, userID int64) (*models.Profile, error) {
	return s.repository.GetByUserID(ctx, userID)
}

func (s *ProfileService) Save(ctx context.Context, userID int64, input SaveProfileInput) (*models.Profile, error) {
	profile, err := buildProfile(userID, input, s.now())
	if err != nil {
		return nil, err
	}
	return s.repository.Upsert(ctx, profile)
}

func (s *ProfileService) Deactivate(ctx context.Context, userID int64) error {
	return s.repository.Deactivate(ctx, userID)
}

func buildProfile(userID int64, input SaveProfileInput, now time.Time) (models.Profile, error) {
	name := strings.TrimSpace(input.FullName)
	if name == "" {
		return models.Profile{}, ErrProfileNameRequired
	}
	birthday, err := time.Parse("2006-01-02", input.Birthday)
	if err != nil || birthday.After(now) {
		return models.Profile{}, ErrInvalidProfileBirthday
	}
	if birthday.AddDate(18, 0, 0).After(now) {
		return models.Profile{}, ErrProfileOwnerMustBeAdult
	}
	if input.Gender != 1 && input.Gender != 2 {
		return models.Profile{}, ErrInvalidProfileGender
	}
	if (input.Latitude == nil) != (input.Longitude == nil) {
		return models.Profile{}, ErrCoordinatesMustBeTogether
	}
	if input.Latitude != nil && (*input.Latitude < -90 || *input.Latitude > 90 || *input.Longitude < -180 || *input.Longitude > 180) {
		return models.Profile{}, ErrInvalidProfileCoordinates
	}

	var city *string
	if input.City != nil {
		value := strings.TrimSpace(*input.City)
		if value != "" {
			city = &value
		}
	}
	return models.Profile{
		UserID:    userID,
		FullName:  name,
		Birthday:  birthday,
		Gender:    input.Gender,
		Bio:       strings.TrimSpace(input.Bio),
		Latitude:  input.Latitude,
		Longitude: input.Longitude,
		City:      city,
	}, nil
}
