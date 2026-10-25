package model

import "time"

// Page status values (pages.status).
const (
	PageStatusDraft     = "draft"
	PageStatusPublished = "published"
)

// Page block types (page_blocks.type).
const (
	PageBlockLink   = "link"
	PageBlockText   = "text"
	PageBlockImage  = "image"
	PageBlockSocial = "social"
)

// Page is a link-in-bio page of a group, served publicly at /m/{slug}.
// Slug is globally unique (case-insensitive) and stored lowercase. Design is a
// JSON document validated by the service layer. PublishedContent is the
// snapshot served publicly; it is nil until the first publish (SQL NULL).
// Revision counts editor writes (1 on create, +1 per page or block write);
// PublishedRevision is the revision the current snapshot was built from (0
// before the first publish).
type Page struct {
	ID                uint64     `db:"id" json:"id"`
	UserID            uint64     `db:"user_id" json:"user_id"`
	GroupID           uint64     `db:"group_id" json:"group_id"`
	Slug              string     `db:"slug" json:"slug"`
	Title             string     `db:"title" json:"title"`
	Description       string     `db:"description" json:"description"`
	AvatarURL         string     `db:"avatar_url" json:"avatar_url"`
	Design            JSONMap    `db:"design" json:"design"`
	Status            string     `db:"status" json:"status"`
	PublishedContent  JSONMap    `db:"-" json:"published_content,omitempty"`
	PublishedAt       *time.Time `db:"published_at" json:"published_at"`
	Revision          uint64     `db:"revision" json:"revision"`
	PublishedRevision uint64     `db:"published_revision" json:"published_revision"`
	Archived          bool       `db:"archived" json:"archived"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// PageBlock is one ordered block of a page. Content is a JSON document whose
// shape depends on Type (validated by the service layer). LinkID is set only
// for link blocks; the block is deleted with its link (FK cascade).
type PageBlock struct {
	ID        uint64    `db:"id" json:"id"`
	PageID    uint64    `db:"page_id" json:"page_id"`
	Type      string    `db:"type" json:"type"`
	Position  int       `db:"position" json:"position"`
	Content   JSONMap   `db:"content" json:"content"`
	IsActive  bool      `db:"is_active" json:"is_active"`
	IsPinned  bool      `db:"is_pinned" json:"is_pinned"`
	LinkID    *uint64   `db:"link_id" json:"link_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// PageView is one recorded public view of a published page. IPHash is the
// salted hash of the visitor IP (same salt as clicks).
type PageView struct {
	ID         uint64    `db:"id" json:"id"`
	PageID     uint64    `db:"page_id" json:"page_id"`
	ViewedAt   time.Time `db:"viewed_at" json:"viewed_at"`
	IPHash     string    `db:"ip_hash" json:"-"`
	Referrer   string    `db:"referrer" json:"referrer"`
	Country    string    `db:"country" json:"country"`
	City       string    `db:"city" json:"city"`
	DeviceType string    `db:"device_type" json:"device_type"`
	Browser    string    `db:"browser" json:"browser"`
}
