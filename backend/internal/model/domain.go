package model

import "time"

// Domain verification states.
const (
	DomainVerificationPending = "pending"
	DomainVerificationOK      = "ok"
	DomainVerificationError   = "error"
)

// Domain represents a custom domain bound to a user. GroupID nil means the
// domain is usable by all of the user's groups.
type Domain struct {
	ID                 uint64    `json:"id" db:"id"`
	UserID             uint64    `json:"user_id" db:"user_id"`
	GroupID            *uint64   `json:"group_id" db:"group_id"`
	Domain             string    `json:"domain" db:"domain"`
	VerificationToken  string    `json:"verification_token" db:"verification_token"`
	VerificationStatus string    `json:"verification_status" db:"verification_status"`
	VerificationError  *string   `json:"verification_error" db:"verification_error"`
	VerifiedAt         NullTime  `json:"verified_at" db:"verified_at"`
	LastCheckedAt      NullTime  `json:"last_checked_at" db:"last_checked_at"`
	RootRedirect       *string   `json:"root_redirect" db:"root_redirect"`
	NotFoundRedirect   *string   `json:"not_found_redirect" db:"not_found_redirect"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}
