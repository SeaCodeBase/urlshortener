package model

import "time"

// Domain represents a custom domain bound to a user
type Domain struct {
	ID        uint64    `json:"id" db:"id"`
	UserID    uint64    `json:"user_id" db:"user_id"`
	Domain    string    `json:"domain" db:"domain"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
