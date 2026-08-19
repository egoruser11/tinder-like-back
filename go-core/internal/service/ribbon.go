package service

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"tinder-core/internal/events"
	"tinder-core/internal/geo"
	"tinder-core/internal/models"
	"tinder-core/internal/repository"
)

var (
	ErrInvalidFeedCursor          = errors.New("invalid feed cursor")
	ErrInvalidFeedLimit           = errors.New("invalid feed limit")
	ErrProfileCoordinatesRequired = errors.New("profile coordinates are required")
	ErrInvalidTargetUser          = errors.New("invalid target user")
	ErrInvalidReportReason        = errors.New("invalid report reason")
	ErrReportCommentTooLong       = errors.New("report comment is too long")
	ErrInvalidDiscoveryAge        = errors.New("invalid discovery age range")
	ErrInvalidDiscoveryGender     = errors.New("invalid discovery gender")
	ErrInvalidDiscoveryDistance   = errors.New("invalid discovery distance")
)

const (
	defaultFeedLimit          = 20
	maximumFeedLimit          = 50
	preliminarySearchRadiusKM = 100
)

type ribbonStore interface {
	GetFilters(context.Context, int64) (*models.DiscoveryPreferences, error)
	UpsertFilters(context.Context, models.DiscoveryPreferences) (*models.DiscoveryPreferences, error)
	ListCandidates(context.Context, repository.CandidateQuery) ([]models.DiscoveryCandidate, error)
	ListIncomingLikes(context.Context, int64, int64, int) ([]models.DiscoveryCandidate, error)
	CreateLike(context.Context, int64, int64) (*int64, error)
	CreateDislike(context.Context, int64, int64) error
	CreateBlock(context.Context, int64, int64) error
	RemoveBlock(context.Context, int64, int64) error
	CreateReport(context.Context, int64, int64, int16, *string) error
}

type profileReader interface {
	GetByUserID(context.Context, int64) (*models.Profile, error)
}

type eventPublisher interface {
	Publish(events.Event)
}

// RibbonService owns discovery operations: feed, likes, dislikes, blocks,
// reports, preferences, and match creation.
type RibbonService struct {
	ribbonRepository  ribbonStore
	profileRepository profileReader
	publisher         eventPublisher
	logger            *slog.Logger
}

func NewRibbonService(
	ribbonRepository ribbonStore,
	profileRepository profileReader,
	publisher eventPublisher,
	logger *slog.Logger,
) *RibbonService {
	return &RibbonService{
		ribbonRepository:  ribbonRepository,
		profileRepository: profileRepository,
		publisher:         publisher,
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

type SavePreferencesInput struct {
	City          *string `json:"city"`
	MinAge        int16   `json:"min_age"`
	MaxAge        int16   `json:"max_age"`
	Gender        *int16  `json:"gender"`
	IsVerified    bool    `json:"is_verified"`
	MaxDistanceKM int32   `json:"max_distance_km"`
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

	boundsRadiusKM := math.Max(preliminarySearchRadiusKM, float64(filters.MaxDistanceKM))
	bounds, err := geo.NewBounds(*profile.Latitude, *profile.Longitude, boundsRadiusKM)
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

func (s *RibbonService) GetPreferences(ctx context.Context, userID int64) (*models.DiscoveryPreferences, error) {
	return s.ribbonRepository.GetFilters(ctx, userID)
}

func (s *RibbonService) SavePreferences(ctx context.Context, userID int64, input SavePreferencesInput) (*models.DiscoveryPreferences, error) {
	preferences, err := buildPreferences(userID, input)
	if err != nil {
		return nil, err
	}
	return s.ribbonRepository.UpsertFilters(ctx, preferences)
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
	s.publish("swipe", userID, input.TargetUserID, map[string]any{"liked": true})
	if chatID != nil {
		s.publish("match", userID, input.TargetUserID, map[string]any{"chat_id": *chatID})
	}
	return LikeOutput{Matched: chatID != nil, ChatID: chatID}, nil
}

// Dislike will exclude a user from the current user's feed.
func (s *RibbonService) Dislike(ctx context.Context, userID int64, input TargetInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	if err := s.ribbonRepository.CreateDislike(ctx, userID, input.TargetUserID); err != nil {
		return err
	}
	s.publish("swipe", userID, input.TargetUserID, map[string]any{"liked": false})
	return nil
}

// Block will permanently exclude a user and deactivate their chat if needed.
func (s *RibbonService) Block(ctx context.Context, userID int64, input TargetInput) error {
	if err := validateTarget(userID, input.TargetUserID); err != nil {
		return err
	}
	if err := s.ribbonRepository.CreateBlock(ctx, userID, input.TargetUserID); err != nil {
		return err
	}
	s.publish("block", userID, input.TargetUserID, nil)
	return nil
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
	if err := s.ribbonRepository.CreateReport(ctx, userID, input.TargetUserID, input.Reason, commentPointer); err != nil {
		return err
	}
	s.publish("report", userID, input.TargetUserID, map[string]any{"reason": input.Reason})
	return nil
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

func buildPreferences(userID int64, input SavePreferencesInput) (models.DiscoveryPreferences, error) {
	if input.MinAge < 18 || input.MaxAge < input.MinAge || input.MaxAge > 99 {
		return models.DiscoveryPreferences{}, ErrInvalidDiscoveryAge
	}
	if input.Gender != nil && *input.Gender != 1 && *input.Gender != 2 {
		return models.DiscoveryPreferences{}, ErrInvalidDiscoveryGender
	}
	if input.MaxDistanceKM <= 0 || input.MaxDistanceKM > 1000 {
		return models.DiscoveryPreferences{}, ErrInvalidDiscoveryDistance
	}

	var city *string
	if input.City != nil {
		value := strings.TrimSpace(*input.City)
		if value != "" {
			city = &value
		}
	}
	return models.DiscoveryPreferences{
		UserID:        userID,
		City:          city,
		MinAge:        input.MinAge,
		MaxAge:        input.MaxAge,
		Gender:        input.Gender,
		IsVerified:    input.IsVerified,
		MaxDistanceKM: input.MaxDistanceKM,
	}, nil
}

func (s *RibbonService) publish(eventType string, userID, targetUserID int64, payload map[string]any) {
	if s.publisher == nil {
		return
	}
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["user_id"] = userID
	payload["target_user_id"] = targetUserID
	payload["occurred_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	s.publisher.Publish(events.Event{Type: eventType, Payload: payload})
}
