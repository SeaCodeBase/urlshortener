package model

import "time"

type User struct {
	ID                     uint64    `db:"id" json:"id"`
	Email                  string    `db:"email" json:"email"`
	DisplayName            *string   `db:"display_name" json:"display_name,omitempty"`
	PasswordHash           string    `db:"password_hash" json:"-"`
	DefaultGroupID         *uint64   `db:"default_group_id" json:"default_group_id"`
	AccountName            *string   `db:"account_name" json:"account_name"`
	HomeChecklistDismissed bool      `db:"home_checklist_dismissed" json:"home_checklist_dismissed"`
	CreatedAt              time.Time `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at" json:"updated_at"`
}
