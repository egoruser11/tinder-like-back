package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tinder-core/internal/models"
)

var ErrProfileNotFound = errors.New("profile not found")

// ProfileRepository provides access to a user's own profile data. It is kept
// separate from RibbonRepository because profile lifecycle and discovery are
// different application concerns.
type ProfileRepository struct {
	db *pgxpool.Pool
}

func NewProfileRepository(db *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// GetByUserID returns the profile belonging to userID, including the current
// coordinates needed to build a discovery search area.
func (repo *ProfileRepository) GetByUserID(ctx context.Context, userID int64) (*models.Profile, error) {
	var profile models.Profile
	err := repo.db.QueryRow(ctx, `
		SELECT
			id, user_id, full_name, birthday, gender, bio,
			latitude, longitude, city, is_active, last_activity_at,
			created_at, updated_at
		FROM profiles
		WHERE user_id = $1
	`, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.FullName,
		&profile.Birthday,
		&profile.Gender,
		&profile.Bio,
		&profile.Latitude,
		&profile.Longitude,
		&profile.City,
		&profile.IsActive,
		&profile.LastActivityAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get profile for user %d: %w", userID, err)
	}

	return &profile, nil
}
