package model

import "time"

// Group is a workspace owned by a user; links, domains and activity are
// scoped by group.
type Group struct {
	ID              uint64    `db:"id" json:"id"`
	UserID          uint64    `db:"user_id" json:"user_id"`
	Name            string    `db:"name" json:"name"`
	DefaultDomainID *uint64   `db:"default_domain_id" json:"default_domain_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}
