package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/SeaCodeBase/urlshortener/internal/model"
)

// Compile-time check: ClickRepositoryImpl implements ClickRepository
var _ ClickRepository = (*ClickRepositoryImpl)(nil)

type ClickRepositoryImpl struct {
	db *sqlx.DB
}

func NewClickRepository(db *sqlx.DB) *ClickRepositoryImpl {
	return &ClickRepositoryImpl{db: db}
}

func (r *ClickRepositoryImpl) BatchInsert(ctx context.Context, clicks []model.Click) error {
	if len(clicks) == 0 {
		return nil
	}

	query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`

	_, err := r.db.NamedExecContext(ctx, query, clicks)
	return err
}

func (r *ClickRepositoryImpl) GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &count, query, linkID)
	return count, err
}

func (r *ClickRepositoryImpl) GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error) {
	var stats ClickStats
	query := `SELECT COUNT(*) as total_clicks, COUNT(DISTINCT ip_hash) as unique_visitors FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &stats, query, linkID)
	return &stats, err
}

func (r *ClickRepositoryImpl) GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error) {
	var stats []DailyClickStats
	query := `SELECT DATE(clicked_at) as date, COUNT(*) as clicks FROM clicks WHERE link_id = ? AND clicked_at >= DATE_SUB(NOW(), INTERVAL ? DAY) GROUP BY DATE(clicked_at) ORDER BY date DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID, days)
	return stats, err
}

func (r *ClickRepositoryImpl) GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error) {
	var stats []ReferrerStats
	query := `SELECT COALESCE(NULLIF(referrer, ''), 'Direct') as referrer, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY referrer ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	return stats, err
}

func (r *ClickRepositoryImpl) GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error) {
	var stats []DeviceStats
	query := `SELECT device_type, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY device_type ORDER BY count DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}

func (r *ClickRepositoryImpl) GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error) {
	var stats []BrowserStats
	query := `SELECT COALESCE(NULLIF(browser, ''), 'Unknown') as browser, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY browser ORDER BY count DESC LIMIT 10`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}
