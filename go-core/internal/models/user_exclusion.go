package models

import "time"

// UserExclusion соответствует таблице user_exclusions: dislike либо block.
type UserExclusion struct {
	ID             int64     `json:"id"`
	ActionUserID   int64     `json:"action_user_id"`
	TargetUserID   int64     `json:"target_user_id"`
	IsPermanentBan bool      `json:"is_permanent_ban"`
	CreatedAt      time.Time `json:"created_at"`
}
