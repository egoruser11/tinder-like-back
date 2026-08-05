package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

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
