package models

import "time"

// Report соответствует таблице reports.
type Report struct {
	ID           int64     `json:"id"`
	ActionUserID int64     `json:"action_user_id"`
	TargetUserID int64     `json:"target_user_id"`
	Reason       int16     `json:"reason"`
	Comment      *string   `json:"comment"`
	IsResolved   bool      `json:"is_resolved"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
