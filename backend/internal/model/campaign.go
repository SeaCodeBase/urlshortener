package model

import "time"

// Campaign groups a group's links into channels for reporting. Channels are
// group-scoped and shared by campaigns (campaign_channels); a link belongs to
// a campaign through a channel (campaign_links). AutoUTM makes links created
// inside the campaign carry UTM parameters.
type Campaign struct {
	ID          uint64    `db:"id" json:"id"`
	UserID      uint64    `db:"user_id" json:"user_id"`
	GroupID     uint64    `db:"group_id" json:"group_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	AutoUTM     bool      `db:"auto_utm" json:"auto_utm"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// CampaignSummary is a campaign list row: the number of channels attached to
// the campaign and of distinct links in it.
type CampaignSummary struct {
	Campaign
	ChannelCount int `db:"channel_count" json:"channel_count"`
	LinkCount    int `db:"link_count" json:"link_count"`
}

// Channel is a named traffic source (e.g. "Email") unique per group.
type Channel struct {
	ID        uint64    `db:"id" json:"id"`
	GroupID   uint64    `db:"group_id" json:"group_id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// CampaignLink is a link's membership in a campaign channel.
type CampaignLink struct {
	CampaignID uint64    `db:"campaign_id" json:"campaign_id"`
	ChannelID  uint64    `db:"channel_id" json:"channel_id"`
	LinkID     uint64    `db:"link_id" json:"link_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
