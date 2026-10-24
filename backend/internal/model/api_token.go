package model

import "time"

// APIToken is a personal access token. Only the SHA-256 hash of the raw token
// is stored; TokenPrefix is shown to the user to identify it.
type APIToken struct {
	ID          uint64    `db:"id" json:"id"`
	UserID      uint64    `db:"user_id" json:"user_id"`
	Name        string    `db:"name" json:"name"`
	TokenHash   string    `db:"token_hash" json:"-"`
	TokenPrefix string    `db:"token_prefix" json:"prefix"`
	LastUsedAt  NullTime  `db:"last_used_at" json:"last_used_at"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	RevokedAt   NullTime  `db:"revoked_at" json:"revoked_at"`
}
