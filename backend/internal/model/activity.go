package model

import "time"

// ActivityEntry is one row of the per-group activity log.
type ActivityEntry struct {
	ID         uint64    `db:"id" json:"id"`
	UserID     uint64    `db:"user_id" json:"user_id"`
	GroupID    uint64    `db:"group_id" json:"group_id"`
	Action     string    `db:"action" json:"action"`
	EntityType string    `db:"entity_type" json:"entity_type"`
	EntityID   uint64    `db:"entity_id" json:"entity_id"`
	Metadata   JSONMap   `db:"metadata" json:"metadata"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
