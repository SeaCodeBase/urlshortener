package model

import "time"

type User struct {
	ID           uint64    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	DisplayName  *string   `db:"display_name" json:"display_name,omitempty"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
