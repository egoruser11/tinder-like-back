package models

import "time"

// Profile соответствует таблице profiles из миграции 0002_create_profiles.sql.
// Указатели означают nullable-поля в Postgres.
type Profile struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	FullName         string    `json:"full_name"`
	Birthday         time.Time `json:"birthday"`
	Bio              *string   `json:"bio"`
	Gender           int16     `json:"gender"`
	Status           *string   `json:"status"`
	MainPhotoID      *int64    `json:"main_photo_id"`
	PhotoStorageSlot *int64    `json:"photo_storage_slot"`
	PhotoIDs         []int64   `json:"photo_ids"`
	Latitude         *float64  `json:"latitude"`
	Longitude        *float64  `json:"longitude"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
