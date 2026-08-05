package models

import "time"

// DiscoveryPreferences соответствует таблице discovery_preferences.
// Nil в City или Gender означает, что соответствующий фильтр не применяется.
type DiscoveryPreferences struct {
	UserID        int64     `json:"user_id"`
	City          *string   `json:"city"`
	MinAge        int16     `json:"min_age"`
	MaxAge        int16     `json:"max_age"`
	Gender        *int16    `json:"gender"`
	IsVerified    bool      `json:"is_verified"`
	MaxDistanceKM int32     `json:"max_distance_km"`
	UpdatedAt     time.Time `json:"updated_at"`
}
