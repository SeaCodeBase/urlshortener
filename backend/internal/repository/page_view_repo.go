package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var _ PageViewRepository = (*PageViewRepositoryImpl)(nil)

// pageViewFacets are the breakdown facets page_views supports (no os column).
var pageViewFacets = map[string]bool{"referrer": true, "device": true, "country": true, "city": true, "browser": true}

type PageViewRepositoryImpl struct {
	db *sqlx.DB
}

func NewPageViewRepository(db *sqlx.DB) *PageViewRepositoryImpl {
	return &PageViewRepositoryImpl{db: db}
}

func (r *PageViewRepositoryImpl) Insert(ctx context.Context, v *model.PageView) error {
	if v.ViewedAt.IsZero() {
		v.ViewedAt = time.Now().UTC().Truncate(time.Second)
	}
	res, err := r.db.NamedExecContext(ctx, `INSERT INTO page_views (page_id, viewed_at, ip_hash, referrer, country, city, device_type, browser)
		VALUES (:page_id, :viewed_at, :ip_hash, :referrer, :country, :city, :device_type, :browser)`, v)
	if err != nil {
		logger.Error(ctx, "page-view-repo: failed to insert view", zap.Uint64("page_id", v.PageID), zap.Error(err))
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	v.ID = uint64(id)
	return nil
}

func (r *PageViewRepositoryImpl) CountAllTime(ctx context.Context, pageID uint64) (int64, error) {
	var n int64
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM page_views WHERE page_id = ?`, pageID); err != nil {
		logger.Error(ctx, "page-view-repo: failed to count all time", zap.Uint64("page_id", pageID), zap.Error(err))
		return 0, err
	}
	return n, nil
}

// pageRangeWhere selects the page's views over the inclusive UTC date range.
func pageRangeWhere(pageID uint64, from, to time.Time) (string, []any) {
	start, end := halfOpen(from, to)
	return `page_id = ? AND viewed_at >= ? AND viewed_at < ?`, []any{pageID, start, end}
}

func (r *PageViewRepositoryImpl) CountRange(ctx context.Context, pageID uint64, from, to time.Time) (int64, error) {
	where, args := pageRangeWhere(pageID, from, to)
	var n int64
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM page_views WHERE `+where, args...); err != nil {
		logger.Error(ctx, "page-view-repo: failed to count range", zap.Uint64("page_id", pageID), zap.Error(err))
		return 0, err
	}
	return n, nil
}

func (r *PageViewRepositoryImpl) Series(ctx context.Context, pageID uint64, from, to time.Time, unit string) ([]Bucket, error) {
	u, ok := seriesUnits[unit]
	if !ok {
		return nil, fmt.Errorf("page-view-repo: unknown series unit %q", unit)
	}
	where, args := pageRangeWhere(pageID, from, to)
	var rows []Bucket
	query := `SELECT ` + u.bucketExpr("viewed_at") + ` AS bucket, COUNT(*) AS cnt
			  FROM page_views WHERE ` + where + ` GROUP BY bucket`
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "page-view-repo: failed to get series", zap.Uint64("page_id", pageID), zap.String("unit", unit), zap.Error(err))
		return nil, err
	}
	return u.zeroFill(rows, from, to), nil
}

func (r *PageViewRepositoryImpl) Breakdown(ctx context.Context, pageID uint64, from, to time.Time, facet string, limit int) ([]Row, error) {
	cols, ok := breakdownColumns[facet]
	if !ok || !pageViewFacets[facet] {
		return nil, fmt.Errorf("page-view-repo: unknown breakdown facet %q", facet)
	}
	where, args := pageRangeWhere(pageID, from, to)
	query := `SELECT ` + cols[0] + ` AS k, ` + cols[1] + ` AS k2, COUNT(*) AS cnt
			  FROM page_views WHERE ` + where + breakdownOrder
	args = append(args, limit)
	rows := []Row{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "page-view-repo: failed to get breakdown", zap.Uint64("page_id", pageID), zap.String("facet", facet), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *PageViewRepositoryImpl) CountsForPages(ctx context.Context, pageIDs []uint64) (map[uint64]int64, error) {
	out := map[uint64]int64{}
	if len(pageIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(`SELECT page_id, COUNT(*) AS cnt FROM page_views WHERE page_id IN (?) GROUP BY page_id`, pageIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		PageID uint64 `db:"page_id"`
		Count  int64  `db:"cnt"`
	}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "page-view-repo: failed to count pages", zap.Int("pages", len(pageIDs)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.PageID] = row.Count
	}
	return out, nil
}
