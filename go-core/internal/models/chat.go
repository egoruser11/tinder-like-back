package models

import "time"

// Chat соответствует таблице chats.
type Chat struct {
	ID            int64      `json:"id"`
	User1ID       int64      `json:"user_1_id"`
	User2ID       int64      `json:"user_2_id"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
	DeactivatedAt *time.Time `json:"deactivated_at"`
}
