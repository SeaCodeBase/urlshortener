package model

import "time"

// QRCode is a QR code that owns exactly one link (qr_codes.link_id is
// unique). Expiration lives on the owned link. RenderCustomizations is stored
// opaquely for the client-side renderer.
type QRCode struct {
	ID                   uint64    `db:"id" json:"id"`
	UserID               uint64    `db:"user_id" json:"user_id"`
	GroupID              uint64    `db:"group_id" json:"group_id"`
	LinkID               uint64    `db:"link_id" json:"link_id"`
	Title                *string   `db:"title" json:"title"`
	RenderCustomizations JSONMap   `db:"render_customizations" json:"render_customizations"`
	Archived             bool      `db:"archived" json:"archived"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
}
