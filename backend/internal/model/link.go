package model

import (
	"database/sql"
	"time"
)

type Link struct {
	ID          uint64       `db:"id" json:"id"`
	UserID      uint64       `db:"user_id" json:"user_id"`
	ShortCode   string       `db:"short_code" json:"short_code"`
	OriginalURL string       `db:"original_url" json:"original_url"`
	Title       *string      `db:"title" json:"title,omitempty"`
	ExpiresAt   sql.NullTime `db:"expires_at" json:"expires_at,omitempty"`
	IsActive    bool         `db:"is_active" json:"is_active"`
	CreatedAt   time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at" json:"updated_at"`
}

type LinkWithStats struct {
	Link
	TotalClicks int64 `json:"total_clicks"`
}
