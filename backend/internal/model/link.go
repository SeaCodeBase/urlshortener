package model

import (
	"time"
)

// Link sources: links created directly, by a QR code, or by a page block.
const (
	LinkSourceLink = "link"
	LinkSourceQR   = "qr"
	LinkSourcePage = "page"
)

type Link struct {
	ID                uint64    `db:"id" json:"id"`
	UserID            uint64    `db:"user_id" json:"user_id"`
	GroupID           uint64    `db:"group_id" json:"group_id"`
	ShortCode         string    `db:"short_code" json:"short_code"`
	OriginalURL       string    `db:"original_url" json:"original_url"`
	LongURLHost       *string   `db:"long_url_host" json:"-"`
	Title             *string   `db:"title" json:"title,omitempty"`
	ExpiresAt         NullTime  `db:"expires_at" json:"expires_at"`
	IsActive          bool      `db:"is_active" json:"is_active"`
	Archived          bool      `db:"archived" json:"archived"`
	HasCustomBackHalf bool      `db:"has_custom_back_half" json:"has_custom_back_half"`
	RedirectCount     int       `db:"redirect_count" json:"redirect_count"`
	ParentLinkID      *uint64   `db:"parent_link_id" json:"parent_link_id,omitempty"`
	Source            string    `db:"source" json:"source"`
	DomainID          *uint64   `db:"domain_id" json:"domain_id,omitempty"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}

type LinkWithStats struct {
	Link
	TotalClicks int64    `db:"total_clicks" json:"total_clicks"`
	Tags        []string `db:"-" json:"tags"`
	QRCodeID    *uint64  `db:"-" json:"qr_code_id,omitempty"`
	FaviconURL  string   `db:"-" json:"favicon_url,omitempty"`
}
