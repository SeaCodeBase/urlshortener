package repository

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
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

	query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, ip_address, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :ip_address, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`

	_, err := r.db.NamedExecContext(ctx, query, clicks)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to batch insert clicks",
			zap.Int("count", len(clicks)),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (r *ClickRepositoryImpl) GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &count, query, linkID)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get total clicks",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
}

func (r *ClickRepositoryImpl) GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error) {
	var stats ClickStats
	query := `SELECT COUNT(*) as total_clicks, COUNT(DISTINCT ip_hash) as unique_visitors FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &stats, query, linkID)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get click stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return &stats, nil
}

func (r *ClickRepositoryImpl) GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error) {
	var stats []DailyClickStats
	query := `SELECT DATE(clicked_at) as date, COUNT(*) as clicks FROM clicks WHERE link_id = ? AND clicked_at >= DATE_SUB(NOW(), INTERVAL ? DAY) GROUP BY DATE(clicked_at) ORDER BY date DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID, days)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get daily stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}

func (r *ClickRepositoryImpl) GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error) {
	var stats []ReferrerStats
	query := `SELECT COALESCE(NULLIF(referrer, ''), 'Direct') as referrer, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY referrer ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get top referrers",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}

func (r *ClickRepositoryImpl) GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error) {
	var stats []DeviceStats
	query := `SELECT device_type, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY device_type ORDER BY count DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get device stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}

func (r *ClickRepositoryImpl) GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error) {
	var stats []BrowserStats
	query := `SELECT COALESCE(NULLIF(browser, ''), 'Unknown') as browser, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY browser ORDER BY count DESC LIMIT 10`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get browser stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}

func (r *ClickRepositoryImpl) GetCountryStats(ctx context.Context, linkID uint64, limit int) ([]CountryStats, error) {
	var stats []CountryStats
	query := `SELECT COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count
			  FROM clicks WHERE link_id = ? GROUP BY country ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get country stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}

func (r *ClickRepositoryImpl) GetCityStats(ctx context.Context, linkID uint64, limit int) ([]CityStats, error) {
	var stats []CityStats
	query := `SELECT COALESCE(NULLIF(city, ''), 'Unknown') as city,
			  COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count
			  FROM clicks WHERE link_id = ? GROUP BY city, country ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	if err != nil {
		logger.Error(ctx, "click-repo: failed to get city stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return stats, nil
}
