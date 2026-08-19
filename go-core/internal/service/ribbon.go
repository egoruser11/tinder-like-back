package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"tinder-core/internal/geo"
	"tinder-core/internal/models"
	"tinder-core/internal/repository"
)

// ErrNotImplemented marks endpoints whose domain algorithm is deliberately
// left for the application author to implement.
var (
	ErrNotImplemented             = errors.New("ribbon operation is not implemented")
	ErrInvalidFeedCursor          = errors.New("invalid feed cursor")
	ErrProfileCoordinatesRequired = errors.New("profile coordinates are required")
)

const (
	defaultFeedLimit          = 20
	maximumFeedLimit          = 50
	preliminarySearchRadiusKM = 100
)

type ribbonStore interface {
	GetFilters(context.Context, int64) (*models.DiscoveryPreferences, error)
	ListCandidates(context.Context, repository.CandidateQuery) ([]models.DiscoveryCandidate, error)
}

type profileStore interface {
	GetByUserID(context.Context, int64) (*models.Profile, error)
}

// RibbonService is the home for all discovery-domain operations: feed,
// likes, dislikes, blocks, reports, and later matches. It has database access
// available, but contains no selection or ranking algorithm yet.
type RibbonService struct {
	ribbonRepository  ribbonStore
	profileRepository profileStore
	logger            *slog.Logger
}

func NewRibbonService(
	ribbonRepository ribbonStore,
	profileRepository profileStore,
	logger *slog.Logger,
) *RibbonService {
	return &RibbonService{
		ribbonRepository:  ribbonRepository,
		profileRepository: profileRepository,
		logger:            logger,
	}
}

type FeedInput struct {
	Limit  int
	Cursor string
}

type FeedOutput struct {
	Items      []FeedItem `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

type FeedItem struct {
	ProfileID  int64   `json:"profile_id"`
	UserID     int64   `json:"user_id"`
	FullName   string  `json:"full_name"`
	Birthday   string  `json:"birthday"`
	Gender     int16   `json:"gender"`
	Bio        string  `json:"bio"`
	City       *string `json:"city,omitempty"`
	IsVerified bool    `json:"is_verified"`
	DistanceKM float64 `json:"distance_km"`
}

type TargetInput struct {
	TargetUserID int64 `json:"target_user_id"`
}

type ReportInput struct {
	TargetUserID int64  `json:"target_user_id"`
	Reason       int16  `json:"reason"`
	Comment      string `json:"comment"`
}

// GetFeed selects candidates in a SQL bounding rectangle and then uses an
// exact Haversine calculation to retain only profiles inside the requested
// distance.
func (s *RibbonService) GetFeed(ctx context.Context, userID int64, input FeedInput) (FeedOutput, error) {
	limit, err := normalizeFeedLimit(input.Limit)
	if err != nil {
		return FeedOutput{}, err
	}
	afterProfileID, err := parseCursor(input.Cursor)
	if err != nil {
		return FeedOutput{}, err
	}

	filters, err := s.ribbonRepository.GetFilters(ctx, userID)
	if err != nil {
		return FeedOutput{}, err
	}
	profile, err := s.profileRepository.GetByUserID(ctx, userID)
	if err != nil {
		return FeedOutput{}, err
	}
	if profile.Latitude == nil || profile.Longitude == nil {
		return FeedOutput{}, ErrProfileCoordinatesRequired
	}

	bounds, err := geo.NewBounds(*profile.Latitude, *profile.Longitude, preliminarySearchRadiusKM)
	if err != nil {
		return FeedOutput{}, err
	}

	// The database pre-filters a rectangle. A larger batch avoids returning a
	// short page solely because some rectangle corners lie outside the circle.
	batchSize := limit * 3
	items := make([]FeedItem, 0, limit)
	scanCursor := afterProfileID
	for len(items) < limit {
		candidates, err := s.ribbonRepository.ListCandidates(ctx, repository.CandidateQuery{
			ViewerUserID:   userID,
			Filters:        *filters,
			Bounds:         bounds,
			AfterProfileID: scanCursor,
			Limit:          batchSize,
		})
		if err != nil {
			return FeedOutput{}, err
		}
		if len(candidates) == 0 {
			break
		}

		for _, candidate := range candidates {
			scanCursor = candidate.Profile.ID
			if candidate.Profile.Latitude == nil || candidate.Profile.Longitude == nil {
				continue
			}
			distanceKM := geo.DistanceKM(*profile.Latitude, *profile.Longitude, *candidate.Profile.Latitude, *candidate.Profile.Longitude)
			if distanceKM > float64(filters.MaxDistanceKM) {
				continue
			}

			items = append(items, FeedItem{
				ProfileID:  candidate.Profile.ID,
				UserID:     candidate.Profile.UserID,
				FullName:   candidate.Profile.FullName,
				Birthday:   candidate.Profile.Birthday.Format("2006-01-02"),
				Gender:     candidate.Profile.Gender,
				Bio:        candidate.Profile.Bio,
				City:       candidate.Profile.City,
				IsVerified: candidate.IsVerified,
				DistanceKM: roundDistance(distanceKM),
			})
			if len(items) == limit {
				break
			}
		}

		if len(items) == limit || len(candidates) < batchSize {
			break
		}
	}

	output := FeedOutput{Items: items}
	if len(items) == limit {
		output.NextCursor = strconv.FormatInt(items[len(items)-1].ProfileID, 10)
	}
	return output, nil
}

// GetIncomingLikes will return people who liked the current user.
func (s *RibbonService) GetIncomingLikes(context.Context, int64) (FeedOutput, error) {
	return FeedOutput{}, ErrNotImplemented
}

// Like will persist a like and, when appropriate, create a match/chat.
func (s *RibbonService) Like(context.Context, int64, TargetInput) error {
	return ErrNotImplemented
}

// Dislike will exclude a user from the current user's feed.
func (s *RibbonService) Dislike(context.Context, int64, TargetInput) error {
	return ErrNotImplemented
}

// Block will permanently exclude a user and deactivate their chat if needed.
func (s *RibbonService) Block(context.Context, int64, TargetInput) error {
	return ErrNotImplemented
}

// Unblock will remove a permanent feed exclusion.
func (s *RibbonService) Unblock(context.Context, int64, TargetInput) error {
	return ErrNotImplemented
}

// Report will store a complaint and later publish an analytics/moderation event.
func (s *RibbonService) Report(context.Context, int64, ReportInput) error {
	return ErrNotImplemented
}

func normalizeFeedLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultFeedLimit, nil
	}
	if limit < 0 || limit > maximumFeedLimit {
		return 0, ErrInvalidFeedCursor
	}
	return limit, nil
}

func parseCursor(cursor string) (int64, error) {
	if strings.TrimSpace(cursor) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || value < 0 {
		return 0, ErrInvalidFeedCursor
	}
	return value, nil
}

func roundDistance(distance float64) float64 {
	return float64(int(distance*100+0.5)) / 100
}
