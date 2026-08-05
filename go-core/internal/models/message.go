package models

import "time"

// Message соответствует таблице messages. DeletedAt реализует мягкое удаление.
type Message struct {
	ID        int64      `json:"id"`
	ChatID    int64      `json:"chat_id"`
	SenderID  int64      `json:"sender_id"`
	Body      string     `json:"body"`
	IsRead    bool       `json:"is_read"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}
