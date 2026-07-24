package domain

import "time"

// User is the base account record. Extend it as you work through
// ticket-2 (schema/repository) and ticket-3 (auth).
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
