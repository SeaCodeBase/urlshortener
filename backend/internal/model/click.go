// backend/internal/model/click.go
package model

import "time"

type Click struct {
	ID          uint64    `db:"id" json:"id"`
	LinkID      uint64    `db:"link_id" json:"link_id"`
	ClickedAt   time.Time `db:"clicked_at" json:"clicked_at"`
	IPHash      string    `db:"ip_hash" json:"-"`
	IPAddress   string    `db:"ip_address" json:"-"`
	UserAgent   string    `db:"user_agent" json:"user_agent"`
	Referrer    string    `db:"referrer" json:"referrer"`
	Country     string    `db:"country" json:"country"`
	City        string    `db:"city" json:"city"`
	DeviceType  string    `db:"device_type" json:"device_type"`
	Browser     string    `db:"browser" json:"browser"`
	UTMSource   string    `db:"utm_source" json:"utm_source"`
	UTMMedium   string    `db:"utm_medium" json:"utm_medium"`
	UTMCampaign string    `db:"utm_campaign" json:"utm_campaign"`
}
