package service

import (
	"context"
	"errors"
	"log/slog"

	"tinder-core/internal/repository"
)

// ErrNotImplemented marks endpoints whose domain algorithm is deliberately
// left for the application author to implement.
var ErrNotImplemented = errors.New("ribbon operation is not implemented")

// RibbonService is the home for all discovery-domain operations: feed,
// likes, dislikes, blocks, reports, and later matches. It has database access
// available, but contains no selection or ranking algorithm yet.
type RibbonService struct {
	ribbonRepository  *repository.RibbonRepository
	profileRepository *repository.ProfileRepository
	logger            *slog.Logger
}

func NewRibbonService(
	ribbonRepository *repository.RibbonRepository,
	profileRepository *repository.ProfileRepository,
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
	UserID int64 `json:"user_id"`
}

type TargetInput struct {
	TargetUserID int64 `json:"target_user_id"`
}

type ReportInput struct {
	TargetUserID int64  `json:"target_user_id"`
	Reason       int16  `json:"reason"`
	Comment      string `json:"comment"`
}

// GetFeed will select, filter, rank, and paginate candidates.
func (s *RibbonService) GetFeed(context.Context, int64, FeedInput) (FeedOutput, error) {
	return FeedOutput{}, ErrNotImplemented
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
