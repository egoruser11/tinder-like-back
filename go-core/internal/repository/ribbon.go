package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinder-core/internal/geo"
	"tinder-core/internal/models"
)

var (
	ErrTargetUserNotFound           = errors.New("target user not found")
	ErrUsersExcluded                = errors.New("users are excluded from each other")
	ErrDiscoveryPreferencesNotFound = errors.New("discovery preferences not found")
)

// RibbonRepository is the future data-access layer for discovery operations.
// It owns no connection itself: the shared application pool is created once in
// main and injected here.
type RibbonRepository struct {
	db *pgxpool.Pool
}

func NewRibbonRepository(db *pgxpool.Pool) *RibbonRepository {
	return &RibbonRepository{db: db}
}

// GetFilters returns the discovery settings saved by a user.
// If the user has not configured filters yet, the method returns pgx.ErrNoRows;
// the service layer can then decide whether to use defaults or ask the client
// to configure them.
func (repo *RibbonRepository) GetFilters(ctx context.Context, userID int64) (*models.DiscoveryPreferences, error) {
	var filters models.DiscoveryPreferences
	err := repo.db.QueryRow(ctx, `
		SELECT user_id, city, min_age, max_age, gender, is_verified, max_distance_km, updated_at
		FROM discovery_preferences
		WHERE user_id = $1
	`, userID).Scan(
		&filters.UserID,
		&filters.City,
		&filters.MinAge,
		&filters.MaxAge,
		&filters.Gender,
		&filters.IsVerified,
		&filters.MaxDistanceKM,
		&filters.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDiscoveryPreferencesNotFound
		}
		return nil, fmt.Errorf("get discovery preferences for user %d: %w", userID, err)
	}

	return &filters, nil
}

// CandidateQuery contains every already validated criterion for a feed query.
// The SQL query additionally applies pair-wise exclusions and prior likes.
type CandidateQuery struct {
	ViewerUserID   int64
	Filters        models.DiscoveryPreferences
	Bounds         geo.Bounds
	AfterProfileID int64
	Limit          int
}

// ListCandidates returns profiles which pass all SQL-level discovery filters.
// The service performs the final Haversine distance check because Bounds is a
// rectangle, not a circle.
func (repo *RibbonRepository) ListCandidates(ctx context.Context, input CandidateQuery) ([]models.DiscoveryCandidate, error) {
	query, args := buildCandidateQuery(input)

	rows, err := repo.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list discovery candidates: %w", err)
	}
	defer rows.Close()

	return scanCandidates(rows)
}

func buildCandidateQuery(input CandidateQuery) (string, []any) {
	conditions := []string{
		"p.is_active = TRUE",
		"p.user_id <> $1",
		"p.latitude IS NOT NULL",
		"p.longitude IS NOT NULL",
		"p.latitude BETWEEN $2 AND $3",
		"p.longitude BETWEEN $4 AND $5",
		"EXTRACT(YEAR FROM age(CURRENT_DATE, p.birthday)) BETWEEN $6 AND $7",
		"p.id > $8",
		`NOT EXISTS (
			SELECT 1
			FROM user_exclusions e
			WHERE (e.action_user_id = $1 AND e.target_user_id = p.user_id)
			   OR (e.action_user_id = p.user_id AND e.target_user_id = $1)
		)`,
		`NOT EXISTS (
			SELECT 1
			FROM swipes s
			WHERE s.actor_user_id = $1
			  AND s.target_user_id = p.user_id
		)`,
	}
	args := []any{
		input.ViewerUserID,
		input.Bounds.MinLatitude,
		input.Bounds.MaxLatitude,
		input.Bounds.MinLongitude,
		input.Bounds.MaxLongitude,
		input.Filters.MinAge,
		input.Filters.MaxAge,
		input.AfterProfileID,
	}

	appendArgument := func(condition string, value any) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, value)
	}
	if input.Filters.City != nil {
		appendArgument("p.city = $%d", *input.Filters.City)
	}
	if input.Filters.Gender != nil {
		appendArgument("p.gender = $%d", *input.Filters.Gender)
	}
	if input.Filters.IsVerified {
		conditions = append(conditions, "u.is_verified = TRUE")
	}
	args = append(args, input.Limit)

	query := `
		SELECT
			p.id, p.user_id, p.full_name, p.birthday, p.gender, p.bio,
			p.latitude, p.longitude, p.city, u.is_verified
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		WHERE ` + strings.Join(conditions, "\n AND ") + `
		ORDER BY p.id ASC
		LIMIT $` + fmt.Sprint(len(args))

	return query, args
}

// ListIncomingLikes returns active profiles that have liked userID and are not
// excluded in either direction.
func (repo *RibbonRepository) ListIncomingLikes(ctx context.Context, userID, afterProfileID int64, limit int) ([]models.DiscoveryCandidate, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			p.id, p.user_id, p.full_name, p.birthday, p.gender, p.bio,
			p.latitude, p.longitude, p.city, u.is_verified
		FROM swipes s
		JOIN profiles p ON p.user_id = s.actor_user_id
		JOIN users u ON u.id = p.user_id
		WHERE s.target_user_id = $1
		  AND p.id > $2
		  AND p.is_active = TRUE
		  AND NOT EXISTS (
			  SELECT 1
			  FROM user_exclusions e
			  WHERE (e.action_user_id = $1 AND e.target_user_id = p.user_id)
			     OR (e.action_user_id = p.user_id AND e.target_user_id = $1)
		  )
		ORDER BY p.id ASC
		LIMIT $3
	`, userID, afterProfileID, limit)
	if err != nil {
		return nil, fmt.Errorf("list incoming likes: %w", err)
	}
	defer rows.Close()

	return scanCandidates(rows)
}

// CreateLike saves a directed like. If the other user already liked the actor,
// it creates (or returns) their single shared chat and returns its ID.
func (repo *RibbonRepository) CreateLike(ctx context.Context, actorUserID, targetUserID int64) (*int64, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create like: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureActiveProfile(ctx, tx, targetUserID); err != nil {
		return nil, err
	}
	if err := ensureNotExcluded(ctx, tx, actorUserID, targetUserID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO swipes (actor_user_id, target_user_id)
		VALUES ($1, $2)
		ON CONFLICT (actor_user_id, target_user_id) DO NOTHING
	`, actorUserID, targetUserID); err != nil {
		return nil, fmt.Errorf("create like: %w", err)
	}

	var mutual bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM swipes
			WHERE actor_user_id = $1 AND target_user_id = $2
		)
	`, targetUserID, actorUserID).Scan(&mutual); err != nil {
		return nil, fmt.Errorf("check mutual like: %w", err)
	}
	if !mutual {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit like: %w", err)
		}
		return nil, nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO chats (user_1_id, user_2_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, actorUserID, targetUserID); err != nil {
		return nil, fmt.Errorf("create chat for match: %w", err)
	}

	var chatID int64
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM chats
		WHERE (user_1_id = $1 AND user_2_id = $2)
		   OR (user_1_id = $2 AND user_2_id = $1)
	`, actorUserID, targetUserID).Scan(&chatID); err != nil {
		return nil, fmt.Errorf("get match chat: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit match: %w", err)
	}
	return &chatID, nil
}

// CreateDislike excludes targetUserID from actorUserID's feed. An existing
// permanent block is never weakened into a temporary dislike.
func (repo *RibbonRepository) CreateDislike(ctx context.Context, actorUserID, targetUserID int64) error {
	if err := repo.ensureUserExists(ctx, targetUserID); err != nil {
		return err
	}
	_, err := repo.db.Exec(ctx, `
		INSERT INTO user_exclusions (action_user_id, target_user_id, is_permanent_ban)
		VALUES ($1, $2, FALSE)
		ON CONFLICT (action_user_id, target_user_id) DO UPDATE
		SET is_permanent_ban = user_exclusions.is_permanent_ban
	`, actorUserID, targetUserID)
	if err != nil {
		return fmt.Errorf("create dislike: %w", err)
	}
	return nil
}

// CreateBlock permanently excludes targetUserID and deactivates the existing
// chat between the pair, if any.
func (repo *RibbonRepository) CreateBlock(ctx context.Context, actorUserID, targetUserID int64) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create block: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureUserExists(ctx, tx, targetUserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_exclusions (action_user_id, target_user_id, is_permanent_ban)
		VALUES ($1, $2, TRUE)
		ON CONFLICT (action_user_id, target_user_id) DO UPDATE
		SET is_permanent_ban = TRUE
	`, actorUserID, targetUserID); err != nil {
		return fmt.Errorf("create block: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE chats
		SET is_active = FALSE, deactivated_at = now()
		WHERE is_active = TRUE
		  AND ((user_1_id = $1 AND user_2_id = $2)
		    OR (user_1_id = $2 AND user_2_id = $1))
	`, actorUserID, targetUserID); err != nil {
		return fmt.Errorf("deactivate blocked chat: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit block: %w", err)
	}
	return nil
}

// RemoveBlock only removes a permanent exclusion created by the current user.
// A temporary dislike is left untouched.
func (repo *RibbonRepository) RemoveBlock(ctx context.Context, actorUserID, targetUserID int64) error {
	_, err := repo.db.Exec(ctx, `
		DELETE FROM user_exclusions
		WHERE action_user_id = $1
		  AND target_user_id = $2
		  AND is_permanent_ban = TRUE
	`, actorUserID, targetUserID)
	if err != nil {
		return fmt.Errorf("remove block: %w", err)
	}
	return nil
}

// CreateReport records a moderation report against another existing user.
func (repo *RibbonRepository) CreateReport(ctx context.Context, actorUserID, targetUserID int64, reason int16, comment *string) error {
	if err := repo.ensureUserExists(ctx, targetUserID); err != nil {
		return err
	}
	_, err := repo.db.Exec(ctx, `
		INSERT INTO reports (action_user_id, target_user_id, reason, comment)
		VALUES ($1, $2, $3, $4)
	`, actorUserID, targetUserID, reason, comment)
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func ensureActiveProfile(ctx context.Context, tx queryRower, userID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE user_id = $1 AND is_active = TRUE)`, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check target profile: %w", err)
	}
	if !exists {
		return ErrTargetUserNotFound
	}
	return nil
}

func ensureNotExcluded(ctx context.Context, tx queryRower, actorUserID, targetUserID int64) error {
	var excluded bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_exclusions
			WHERE (action_user_id = $1 AND target_user_id = $2)
			   OR (action_user_id = $2 AND target_user_id = $1)
		)
	`, actorUserID, targetUserID).Scan(&excluded); err != nil {
		return fmt.Errorf("check user exclusions: %w", err)
	}
	if excluded {
		return ErrUsersExcluded
	}
	return nil
}

func (repo *RibbonRepository) ensureUserExists(ctx context.Context, userID int64) error {
	return ensureUserExists(ctx, repo.db, userID)
}

func ensureUserExists(ctx context.Context, tx queryRower, userID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check target user: %w", err)
	}
	if !exists {
		return ErrTargetUserNotFound
	}
	return nil
}

type candidateRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}

func scanCandidates(rows candidateRows) ([]models.DiscoveryCandidate, error) {
	candidates := make([]models.DiscoveryCandidate, 0)
	for rows.Next() {
		var candidate models.DiscoveryCandidate
		err := rows.Scan(
			&candidate.Profile.ID,
			&candidate.Profile.UserID,
			&candidate.Profile.FullName,
			&candidate.Profile.Birthday,
			&candidate.Profile.Gender,
			&candidate.Profile.Bio,
			&candidate.Profile.Latitude,
			&candidate.Profile.Longitude,
			&candidate.Profile.City,
			&candidate.IsVerified,
		)
		if err != nil {
			return nil, fmt.Errorf("scan discovery candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery candidates: %w", err)
	}
	return candidates, nil
}
