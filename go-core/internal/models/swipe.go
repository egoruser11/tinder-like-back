package models

import "time"

// Swipe соответствует таблице swipes и обозначает поставленный like.
type Swipe struct {
	ID           int64     `json:"id"`
	ActorUserID  int64     `json:"actor_user_id"`
	TargetUserID int64     `json:"target_user_id"`
	CreatedAt    time.Time `json:"created_at"`
}
