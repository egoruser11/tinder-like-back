package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"tinder-core/internal/geo"
	"tinder-core/internal/models"
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

	candidates := make([]models.DiscoveryCandidate, 0, input.Limit)
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
