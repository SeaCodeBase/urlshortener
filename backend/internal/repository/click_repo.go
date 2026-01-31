// backend/internal/repository/click_repo.go
package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/model"
)

type ClickRepository struct {
	db *sqlx.DB
}

func NewClickRepository(db *sqlx.DB) *ClickRepository {
	return &ClickRepository{db: db}
}

func (r *ClickRepository) BatchInsert(ctx context.Context, clicks []model.Click) error {
	if len(clicks) == 0 {
		return nil
	}

	query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`

	_, err := r.db.NamedExecContext(ctx, query, clicks)
	return err
}

func (r *ClickRepository) GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &count, query, linkID)
	return count, err
}

type ClickStats struct {
	TotalClicks    int64 `db:"total_clicks"`
	UniqueVisitors int64 `db:"unique_visitors"`
}

type DailyClickStats struct {
	Date   string `db:"date" json:"date"`
	Clicks int64  `db:"clicks" json:"clicks"`
}

type ReferrerStats struct {
	Referrer string `db:"referrer" json:"referrer"`
	Count    int64  `db:"count" json:"count"`
}

type DeviceStats struct {
	DeviceType string `db:"device_type" json:"device_type"`
	Count      int64  `db:"count" json:"count"`
}

type BrowserStats struct {
	Browser string `db:"browser" json:"browser"`
	Count   int64  `db:"count" json:"count"`
}

func (r *ClickRepository) GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error) {
	var stats ClickStats
	query := `SELECT
		COUNT(*) as total_clicks,
		COUNT(DISTINCT ip_hash) as unique_visitors
		FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &stats, query, linkID)
	return &stats, err
}

func (r *ClickRepository) GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error) {
	var stats []DailyClickStats
	query := `SELECT
		DATE(clicked_at) as date,
		COUNT(*) as clicks
		FROM clicks
		WHERE link_id = ? AND clicked_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		GROUP BY DATE(clicked_at)
		ORDER BY date DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID, days)
	return stats, err
}

func (r *ClickRepository) GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error) {
	var stats []ReferrerStats
	query := `SELECT
		COALESCE(NULLIF(referrer, ''), 'Direct') as referrer,
		COUNT(*) as count
		FROM clicks
		WHERE link_id = ?
		GROUP BY referrer
		ORDER BY count DESC
		LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	return stats, err
}

func (r *ClickRepository) GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error) {
	var stats []DeviceStats
	query := `SELECT
		device_type,
		COUNT(*) as count
		FROM clicks
		WHERE link_id = ?
		GROUP BY device_type
		ORDER BY count DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}

func (r *ClickRepository) GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error) {
	var stats []BrowserStats
	query := `SELECT
		COALESCE(NULLIF(browser, ''), 'Unknown') as browser,
		COUNT(*) as count
		FROM clicks
		WHERE link_id = ?
		GROUP BY browser
		ORDER BY count DESC
		LIMIT 10`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}
