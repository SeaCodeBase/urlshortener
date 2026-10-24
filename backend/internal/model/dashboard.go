package model

import "time"

// Dashboard is a saved analytics dashboard of a group. Filter and Layout are
// JSON documents whose shapes are validated by the service layer; DateRange
// is a preset key such as "last_7_days". Each group has at most one default
// dashboard (IsDefault), which is created on demand and never deleted.
type Dashboard struct {
	ID        uint64    `db:"id" json:"id"`
	GroupID   uint64    `db:"group_id" json:"group_id"`
	CreatedBy uint64    `db:"created_by" json:"created_by"`
	Name      string    `db:"name" json:"name"`
	IsDefault bool      `db:"is_default" json:"is_default"`
	DateRange string    `db:"date_range" json:"date_range"`
	Filter    JSONMap   `db:"filter" json:"filter"`
	Layout    JSONMap   `db:"layout" json:"layout"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
