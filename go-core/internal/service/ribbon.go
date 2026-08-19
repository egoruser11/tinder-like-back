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
	ErrInvalidFeedLimit           = errors.New("invalid feed limit")
	ErrProfileCoordinatesRequired = errors.New("profile coordinates are required")
	ErrInvalidTargetUser          = errors.New("invalid target user")
	ErrInvalidReportReason        = errors.New("invalid report reason")
	ErrReportCommentTooLong       = errors.New("report comment is too long")
)

const (
	defaultFeedLimit          = 20
	maximumFeedLimit          = 50
	preliminarySearchRadiusKM = 100
)

type ribbonStore interface {
	GetFilters(context.Context, int64) (*models.DiscoveryPreferences, error)
	ListCandidates(context.Context, repository.CandidateQuery) ([]models.DiscoveryCandidate, error)
	ListIncomingLikes(context.Context, int64, int64, int) ([]models.DiscoveryCandidate, error)
	CreateLike(context.Context, int64, int64) (*int64, error)
	CreateDislike(context.Context, int64, int64) error
	CreateBlock(context.Context, int64, int64) error
	RemoveBlock(context.Context, int64, int64) error
	CreateReport(context.Context, int64, int64, int16, *string) error
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
	DistanceKM float64 `json:"distance_km,omitempty"`
}

type LikeOutput struct {
	Matched bool   `json:"matched"`
	ChatID  *int64 `json:"chat_id,omitempty"`
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

			items = append(items, feedItemFromCandidate(candidate, distanceKM))
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

// GetIncomingLikes returns active, non-excluded profiles that have liked the
// current user. It does not apply discovery preferences: an incoming like is
// an explicit action, not a feed recommendation.
func (s *RibbonService) GetIncomingLikes(ctx context.Context, userID int64, input FeedInput) (FeedOutput, error) {
	limit, err := normalizeFeedLimit(input.Limit)
	if err != nil {
		return FeedOutput{}, err
	}
	afterProfileID, err := parseCursor(input.Cursor)
	if err != nil {
		return FeedOutput{}, err
	}

	candidates, err := s.ribbonRepository.ListIncomingLikes(ctx, userID, afterProfileID, limit+1)
	if err != nil {
		return FeedOutput{}, err
	}
	output := FeedOutput{Items: make([]FeedItem, 0, min(limit, len(candidates)))}
	for index, candidate := range candidates {
		if index == limit {
			output.NextCursor = strconv.FormatInt(output.Items[len(output.Items)-1].ProfileID, 10)
			break
		}
		output.Items = append(output.Items, feedItemFromCandidate(candidate, 0))
	}
	return output, nil
}

// Like will persist a like and, when appropriate, create a match/chat.
func (s *RibbonService) Like(ctx context.Context, userID int64, input TargetInput) (LikeOutput, error) {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return LikeOutput{}, err
	}
	chatID, err := s.ribbonRepository.CreateLike(ctx, userID, input.TargetUserID)
	if err != nil {
		return LikeOutput{}, err
	}
	return LikeOutput{Matched: chatID != nil, ChatID: chatID}, nil
}

// Dislike will exclude a user from the current user's feed.
func (s *RibbonService) Dislike(ctx context.Context, userID int64, input TargetInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	return s.ribbonRepository.CreateDislike(ctx, userID, input.TargetUserID)
}

// Block will permanently exclude a user and deactivate their chat if needed.
func (s *RibbonService) Block(ctx context.Context, userID int64, input TargetInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	return s.ribbonRepository.CreateBlock(ctx, userID, input.TargetUserID)
}

// Unblock will remove a permanent feed exclusion.
func (s *RibbonService) Unblock(ctx context.Context, userID int64, input TargetInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	return s.ribbonRepository.RemoveBlock(ctx, userID, input.TargetUserID)
}

// Report will store a complaint and later publish an analytics/moderation event.
func (s *RibbonService) Report(ctx context.Context, userID int64, input ReportInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	if input.Reason < 1 || input.Reason > 5 {
		return ErrInvalidReportReason
	}
	comment := strings.TrimSpace(input.Comment)
	if len(comment) > 2000 {
		return ErrReportCommentTooLong
	}
	var commentPointer *string
	if comment != "" {
		commentPointer = &comment
	}
	return s.ribbonRepository.CreateReport(ctx, userID, input.TargetUserID, input.Reason, commentPointer)
}

func normalizeFeedLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultFeedLimit, nil
	}
	if limit < 0 || limit > maximumFeedLimit {
		return 0, ErrInvalidFeedLimit
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

func feedItemFromCandidate(candidate models.DiscoveryCandidate, distanceKM float64) FeedItem {
	return FeedItem{
		ProfileID:  candidate.Profile.ID,
		UserID:     candidate.Profile.UserID,
		FullName:   candidate.Profile.FullName,
		Birthday:   candidate.Profile.Birthday.Format("2006-01-02"),
		Gender:     candidate.Profile.Gender,
		Bio:        candidate.Profile.Bio,
		City:       candidate.Profile.City,
		IsVerified: candidate.IsVerified,
		DistanceKM: roundDistance(distanceKM),
	}
}

func validateTarget(userID, targetUserID int64) error {
	if targetUserID <= 0 || targetUserID == userID {
		return ErrInvalidTargetUser
	}
	return nil
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
