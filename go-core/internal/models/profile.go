package models

import "time"

// Profile соответствует таблице profiles из миграции 0002_create_profiles.sql.
// Указатели означают nullable-поля в Postgres.
type Profile struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	FullName       string    `json:"full_name"`
	Birthday       time.Time `json:"birthday"`
	Gender         int16     `json:"gender"`
	Bio            string    `json:"bio"`
	Latitude       *float64  `json:"latitude"`
	Longitude      *float64  `json:"longitude"`
	City           *string   `json:"city"`
	IsActive       bool      `json:"is_active"`
	LastActivityAt time.Time `json:"last_activity_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
