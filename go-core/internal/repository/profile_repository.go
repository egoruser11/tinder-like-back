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
	err := repo.db.QueryRow(ctx, profileSelectByUserID, userID).Scan(
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

// Upsert creates a profile for a newly registered account or replaces the
// editable profile fields for an existing account. Updating a profile also
// marks it active and refreshes its activity timestamp.
func (repo *ProfileRepository) Upsert(ctx context.Context, profile models.Profile) (*models.Profile, error) {
	var saved models.Profile
	err := repo.db.QueryRow(ctx, `
		INSERT INTO profiles (
			user_id, full_name, birthday, gender, bio, latitude, longitude, city,
			is_active, last_activity_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE, now())
		ON CONFLICT (user_id) DO UPDATE SET
			full_name = EXCLUDED.full_name,
			birthday = EXCLUDED.birthday,
			gender = EXCLUDED.gender,
			bio = EXCLUDED.bio,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			city = EXCLUDED.city,
			is_active = TRUE,
			last_activity_at = now(),
			updated_at = now()
		RETURNING
			id, user_id, full_name, birthday, gender, bio,
			latitude, longitude, city, is_active, last_activity_at,
			created_at, updated_at
	`,
		profile.UserID,
		profile.FullName,
		profile.Birthday,
		profile.Gender,
		profile.Bio,
		profile.Latitude,
		profile.Longitude,
		profile.City,
	).Scan(
		&saved.ID,
		&saved.UserID,
		&saved.FullName,
		&saved.Birthday,
		&saved.Gender,
		&saved.Bio,
		&saved.Latitude,
		&saved.Longitude,
		&saved.City,
		&saved.IsActive,
		&saved.LastActivityAt,
		&saved.CreatedAt,
		&saved.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert profile for user %d: %w", profile.UserID, err)
	}
	return &saved, nil
}

// Deactivate hides the profile from discovery without deleting account data.
func (repo *ProfileRepository) Deactivate(ctx context.Context, userID int64) error {
	commandTag, err := repo.db.Exec(ctx, `
		UPDATE profiles
		SET is_active = FALSE, updated_at = now()
		WHERE user_id = $1 AND is_active = TRUE
	`, userID)
	if err != nil {
		return fmt.Errorf("deactivate profile for user %d: %w", userID, err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrProfileNotFound
	}
	return nil
}

const profileSelectByUserID = `
		SELECT
			id, user_id, full_name, birthday, gender, bio,
			latitude, longitude, city, is_active, last_activity_at,
			created_at, updated_at
		FROM profiles
		WHERE user_id = $1
`
