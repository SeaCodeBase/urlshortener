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

	// INSERT IGNORE skips rows with invalid link_id (e.g., deleted links still in Redis queue)
	// This prevents the entire batch from failing due to a few invalid records
	query := `INSERT IGNORE INTO clicks (link_id, clicked_at, ip_hash, ip_address, user_agent, referrer, country, city, device_type, browser, os, source, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :ip_address, :user_agent, :referrer, :country, :city, :device_type, :browser, :os, :source, :utm_source, :utm_medium, :utm_campaign)`

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

// sourceClause returns the source condition (with a leading AND) for a
// source filter: "" and "all" match everything, "not_qr" excludes QR scans,
// any other value matches that source exactly.
func sourceClause(source string) (string, []any) {
	switch source {
	case "", "all":
		return "", nil
	case "not_qr":
		return ` AND source <> 'qr'`, nil
	default:
		return ` AND source = ?`, []any{source}
	}
}

// scopeWhere builds the WHERE clause for a click scope over the inclusive
// UTC date range. Nil LinkIDs selects every link of the group; a non-nil empty
// slice selects nothing.
func scopeWhere(scope ClickScope, from, to time.Time) (string, []any, error) {
	var where string
	var args []any
	switch {
	case scope.LinkIDs == nil:
		where, args = `link_id IN (SELECT id FROM links WHERE group_id = ?)`, []any{scope.GroupID}
	case len(scope.LinkIDs) == 0:
		where = `1 = 0`
	default:
		q, a, err := sqlx.In(`link_id IN (?)`, scope.LinkIDs)
		if err != nil {
			return "", nil, err
		}
		where, args = q, a
	}
	start, end := halfOpen(from, to)
	where += ` AND clicked_at >= ? AND clicked_at < ?`
	args = append(args, start, end)
	src, srcArgs := sourceClause(scope.Source)
	return where + src, append(args, srcArgs...), nil
}

// linkScope adapts the link-set API, where nil and empty both mean no links.
func linkScope(linkIDs []uint64, source string) ClickScope {
	if linkIDs == nil {
		linkIDs = []uint64{}
	}
	return ClickScope{LinkIDs: linkIDs, Source: source}
}

func (r *ClickRepositoryImpl) CountRange(ctx context.Context, linkIDs []uint64, from, to time.Time, source string) (int64, error) {
	return r.CountScope(ctx, linkScope(linkIDs, source), from, to)
}

func (r *ClickRepositoryImpl) CountRangeByGroup(ctx context.Context, groupID uint64, from, to time.Time, source string) (int64, error) {
	return r.CountScope(ctx, ClickScope{GroupID: groupID, Source: source}, from, to)
}

func (r *ClickRepositoryImpl) CountScope(ctx context.Context, scope ClickScope, from, to time.Time) (int64, error) {
	where, args, err := scopeWhere(scope, from, to)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM clicks WHERE `+where, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to count scope", zap.Uint64("group_id", scope.GroupID), zap.Int("links", len(scope.LinkIDs)), zap.Error(err))
		return 0, err
	}
	return n, nil
}

func (r *ClickRepositoryImpl) TotalAllTime(ctx context.Context, linkIDs []uint64, source string) (int64, error) {
	if len(linkIDs) == 0 {
		return 0, nil
	}
	query, args, err := sqlx.In(`SELECT COUNT(*) FROM clicks WHERE link_id IN (?)`, linkIDs)
	if err != nil {
		return 0, err
	}
	src, srcArgs := sourceClause(source)
	query += src
	args = append(args, srcArgs...)
	var n int64
	if err := r.db.GetContext(ctx, &n, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to count total", zap.Int("links", len(linkIDs)), zap.Error(err))
		return 0, err
	}
	return n, nil
}

func (r *ClickRepositoryImpl) Series(ctx context.Context, linkIDs []uint64, from, to time.Time, unit, source string) ([]Bucket, error) {
	return r.SeriesScope(ctx, linkScope(linkIDs, source), from, to, unit)
}

func (r *ClickRepositoryImpl) SeriesScope(ctx context.Context, scope ClickScope, from, to time.Time, unit string) ([]Bucket, error) {
	u, ok := seriesUnits[unit]
	if !ok {
		return nil, fmt.Errorf("click-repo: unknown series unit %q", unit)
	}
	where, args, err := scopeWhere(scope, from, to)
	if err != nil {
		return nil, err
	}
	var rows []Bucket
	query := `SELECT ` + u.bucketExpr("clicked_at") + ` AS bucket, COUNT(*) AS cnt
			  FROM clicks WHERE ` + where + ` GROUP BY bucket`
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to get series", zap.Uint64("group_id", scope.GroupID), zap.Int("links", len(scope.LinkIDs)), zap.String("unit", unit), zap.Error(err))
		return nil, err
	}
	return u.zeroFill(rows, from, to), nil
}

func (r *ClickRepositoryImpl) Breakdown(ctx context.Context, linkIDs []uint64, from, to time.Time, facet string, limit int, source string) ([]Row, error) {
	return r.BreakdownScope(ctx, linkScope(linkIDs, source), from, to, facet, limit)
}

func (r *ClickRepositoryImpl) BreakdownScope(ctx context.Context, scope ClickScope, from, to time.Time, facet string, limit int) ([]Row, error) {
	cols, ok := breakdownColumns[facet]
	if !ok {
		return nil, fmt.Errorf("click-repo: unknown breakdown facet %q", facet)
	}
	where, args, err := scopeWhere(scope, from, to)
	if err != nil {
		return nil, err
	}
	query := `SELECT ` + cols[0] + ` AS k, ` + cols[1] + ` AS k2, COUNT(*) AS cnt
			  FROM clicks WHERE ` + where + breakdownOrder
	args = append(args, limit)
	rows := []Row{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to get breakdown", zap.String("facet", facet), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *ClickRepositoryImpl) ItemsScope(ctx context.Context, scope ClickScope, from, to time.Time, limit int) ([]ItemCount, error) {
	where, args, err := scopeWhere(scope, from, to)
	if err != nil {
		return nil, err
	}
	query := `SELECT link_id, source, COUNT(*) AS cnt
			  FROM clicks WHERE ` + where + `
			  GROUP BY link_id, source ORDER BY cnt DESC, link_id ASC, source ASC LIMIT ?`
	args = append(args, limit)
	items := []ItemCount{}
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to get items", zap.Uint64("group_id", scope.GroupID), zap.Int("links", len(scope.LinkIDs)), zap.Error(err))
		return nil, err
	}
	return items, nil
}
