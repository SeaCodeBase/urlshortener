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

// utcDay truncates t to midnight of its UTC day.
func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// halfOpen turns inclusive UTC dates into the [start, end) instant range.
func halfOpen(from, to time.Time) (time.Time, time.Time) {
	return utcDay(from), utcDay(to).AddDate(0, 0, 1)
}

func sourceFiltered(source string) bool { return source != "" && source != "all" }

// clickWhere builds the shared WHERE clause for link set, range and source.
func clickWhere(linkIDs []uint64, from, to time.Time, source string) (string, []any, error) {
	start, end := halfOpen(from, to)
	where, args, err := sqlx.In(`link_id IN (?) AND clicked_at >= ? AND clicked_at < ?`, linkIDs, start, end)
	if err != nil {
		return "", nil, err
	}
	if sourceFiltered(source) {
		where += ` AND source = ?`
		args = append(args, source)
	}
	return where, args, nil
}

func (r *ClickRepositoryImpl) CountRange(ctx context.Context, linkIDs []uint64, from, to time.Time, source string) (int64, error) {
	if len(linkIDs) == 0 {
		return 0, nil
	}
	where, args, err := clickWhere(linkIDs, from, to, source)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM clicks WHERE `+where, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to count range", zap.Int("links", len(linkIDs)), zap.Error(err))
		return 0, err
	}
	return n, nil
}

func (r *ClickRepositoryImpl) CountRangeByGroup(ctx context.Context, groupID uint64, from, to time.Time, source string) (int64, error) {
	start, end := halfOpen(from, to)
	query := `SELECT COUNT(*) FROM clicks c JOIN links l ON l.id = c.link_id
			  WHERE l.group_id = ? AND c.clicked_at >= ? AND c.clicked_at < ?`
	args := []any{groupID, start, end}
	if sourceFiltered(source) {
		query += ` AND c.source = ?`
		args = append(args, source)
	}
	var n int64
	if err := r.db.GetContext(ctx, &n, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to count group range", zap.Uint64("group_id", groupID), zap.Error(err))
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
	if sourceFiltered(source) {
		query += ` AND source = ?`
		args = append(args, source)
	}
	var n int64
	if err := r.db.GetContext(ctx, &n, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to count total", zap.Int("links", len(linkIDs)), zap.Error(err))
		return 0, err
	}
	return n, nil
}

// seriesUnits maps a unit to its SQL DATE_FORMAT pattern, Go layout and step.
var seriesUnits = map[string]struct {
	sqlFormat string
	goLayout  string
	step      time.Duration
}{
	"day":  {"%Y-%m-%d", "2006-01-02", 24 * time.Hour},
	"hour": {"%Y-%m-%dT%H:00", "2006-01-02T15:00", time.Hour},
}

func (r *ClickRepositoryImpl) Series(ctx context.Context, linkIDs []uint64, from, to time.Time, unit, source string) ([]Bucket, error) {
	u, ok := seriesUnits[unit]
	if !ok {
		return nil, fmt.Errorf("click-repo: unknown series unit %q", unit)
	}
	counts := map[string]int64{}
	if len(linkIDs) > 0 {
		where, args, err := clickWhere(linkIDs, from, to, source)
		if err != nil {
			return nil, err
		}
		var rows []Bucket
		query := `SELECT DATE_FORMAT(clicked_at, '` + u.sqlFormat + `') AS bucket, COUNT(*) AS cnt
				  FROM clicks WHERE ` + where + ` GROUP BY bucket`
		if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
			logger.Error(ctx, "click-repo: failed to get series", zap.Int("links", len(linkIDs)), zap.String("unit", unit), zap.Error(err))
			return nil, err
		}
		for _, row := range rows {
			counts[row.Date] = row.Count
		}
	}
	start, end := halfOpen(from, to)
	out := []Bucket{}
	for t := start; t.Before(end); t = t.Add(u.step) {
		key := t.Format(u.goLayout)
		out = append(out, Bucket{Date: key, Count: counts[key]})
	}
	return out, nil
}

// breakdownColumns maps a facet to its key and secondary key expressions.
var breakdownColumns = map[string][2]string{
	"country":  {`COALESCE(NULLIF(country, ''), 'Unknown')`, `''`},
	"city":     {`COALESCE(NULLIF(city, ''), 'Unknown')`, `COALESCE(NULLIF(country, ''), 'Unknown')`},
	"referrer": {`COALESCE(NULLIF(referrer, ''), 'Direct')`, `''`},
	"device":   {`CASE WHEN device_type IS NULL OR device_type IN ('', 'unknown') THEN 'Unknown' ELSE device_type END`, `''`},
	"browser":  {`COALESCE(NULLIF(browser, ''), 'Unknown')`, `''`},
	"os":       {`COALESCE(NULLIF(os, ''), 'Unknown')`, `''`},
}

func (r *ClickRepositoryImpl) Breakdown(ctx context.Context, linkIDs []uint64, from, to time.Time, facet string, limit int, source string) ([]Row, error) {
	cols, ok := breakdownColumns[facet]
	if !ok {
		return nil, fmt.Errorf("click-repo: unknown breakdown facet %q", facet)
	}
	if len(linkIDs) == 0 {
		return []Row{}, nil
	}
	where, args, err := clickWhere(linkIDs, from, to, source)
	if err != nil {
		return nil, err
	}
	query := `SELECT ` + cols[0] + ` AS k, ` + cols[1] + ` AS k2, COUNT(*) AS cnt
			  FROM clicks WHERE ` + where + `
			  GROUP BY k, k2 ORDER BY cnt DESC, k ASC, k2 ASC LIMIT ?`
	args = append(args, limit)
	rows := []Row{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "click-repo: failed to get breakdown", zap.String("facet", facet), zap.Error(err))
		return nil, err
	}
	return rows, nil
}
