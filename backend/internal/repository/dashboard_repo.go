package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrDashboardNotFound = errors.New("dashboard not found")

// Sort keys of DashboardRepository.ListByGroup.
const (
	DashboardSortCreated  = "created"
	DashboardSortModified = "modified"
	DashboardSortName     = "name"
)

// Values of the dashboard EnsureDefault creates.
const (
	DefaultDashboardName      = "Default"
	DefaultDashboardDateRange = "last_7_days"
)

// mysqlDeadlock is MySQL/MariaDB error ER_LOCK_DEADLOCK.
const mysqlDeadlock = 1213

// Compile-time check: DashboardRepositoryImpl implements DashboardRepository
var _ DashboardRepository = (*DashboardRepositoryImpl)(nil)

type DashboardRepositoryImpl struct {
	db *sqlx.DB
}

func NewDashboardRepository(db *sqlx.DB) *DashboardRepositoryImpl {
	return &DashboardRepositoryImpl{db: db}
}

const dashboardSelect = `SELECT id, group_id, created_by, name, is_default, date_range, filter, layout,
	created_at, updated_at FROM dashboards`

// defaultFilter is stored when a dashboard has no filter.
func defaultFilter() model.JSONMap { return model.JSONMap{"type": "all"} }

func (r *DashboardRepositoryImpl) Create(ctx context.Context, d *model.Dashboard) error {
	if d.Filter == nil {
		d.Filter = defaultFilter()
	}
	if d.Layout == nil {
		d.Layout = model.JSONMap{}
	}
	if d.DateRange == "" {
		d.DateRange = DefaultDashboardDateRange
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO dashboards (group_id, created_by, name, is_default, date_range, filter, layout)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.GroupID, d.CreatedBy, d.Name, d.IsDefault, d.DateRange, d.Filter, d.Layout)
	if err != nil {
		logger.Error(ctx, "dashboard-repo: failed to create dashboard", zap.Uint64("group_id", d.GroupID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	d.ID = uint64(id)
	var ts struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	if err := r.db.GetContext(ctx, &ts, `SELECT created_at, updated_at FROM dashboards WHERE id = ?`, d.ID); err != nil {
		logger.Error(ctx, "dashboard-repo: failed to read created dashboard", zap.Uint64("dashboard_id", d.ID), zap.Error(err))
		return err
	}
	d.CreatedAt, d.UpdatedAt = ts.CreatedAt, ts.UpdatedAt
	return nil
}

func (r *DashboardRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Dashboard, error) {
	var d model.Dashboard
	err := r.db.GetContext(ctx, &d, dashboardSelect+` WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDashboardNotFound
	}
	if err != nil {
		logger.Error(ctx, "dashboard-repo: failed to get dashboard", zap.Uint64("dashboard_id", id), zap.Error(err))
		return nil, err
	}
	return &d, nil
}

func (r *DashboardRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64, sort string) ([]model.Dashboard, error) {
	order := "created_at DESC, id DESC"
	switch sort {
	case DashboardSortModified:
		order = "updated_at DESC, id DESC"
	case DashboardSortName:
		order = "name ASC, id ASC"
	}
	rows := []model.Dashboard{}
	if err := r.db.SelectContext(ctx, &rows, dashboardSelect+` WHERE group_id = ? ORDER BY `+order, groupID); err != nil {
		logger.Error(ctx, "dashboard-repo: failed to list dashboards", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *DashboardRepositoryImpl) CountByGroup(ctx context.Context, groupID uint64) (int64, error) {
	var count int64
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM dashboards WHERE group_id = ?`, groupID); err != nil {
		logger.Error(ctx, "dashboard-repo: failed to count dashboards", zap.Uint64("group_id", groupID), zap.Error(err))
		return 0, err
	}
	return count, nil
}

func (r *DashboardRepositoryImpl) Update(ctx context.Context, d *model.Dashboard) error {
	if d.Filter == nil {
		d.Filter = defaultFilter()
	}
	if d.Layout == nil {
		d.Layout = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE dashboards SET name = ?, date_range = ?, filter = ?, layout = ? WHERE id = ?`,
		d.Name, d.DateRange, d.Filter, d.Layout, d.ID)
	if err != nil {
		logger.Error(ctx, "dashboard-repo: failed to update dashboard", zap.Uint64("dashboard_id", d.ID), zap.Error(err))
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// 0 affected rows also means "nothing changed"; only a missing row
		// is "not found".
		var exists int
		if err := r.db.GetContext(ctx, &exists, `SELECT COUNT(*) FROM dashboards WHERE id = ?`, d.ID); err != nil {
			return err
		}
		if exists == 0 {
			return ErrDashboardNotFound
		}
	}
	return nil
}

func (r *DashboardRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM dashboards WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "dashboard-repo: failed to delete dashboard", zap.Uint64("dashboard_id", id), zap.Error(err))
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDashboardNotFound
	}
	return nil
}

// EnsureDefault inserts the default dashboard only when the group has none
// (a single INSERT ... WHERE NOT EXISTS), then returns the group's default
// row. Concurrent callers may deadlock on the gap lock the NOT EXISTS check
// takes; the losing insert is rolled back by the server and the winner's row
// is returned instead.
func (r *DashboardRepositoryImpl) EnsureDefault(ctx context.Context, groupID, userID uint64, layout model.JSONMap) (*model.Dashboard, error) {
	if layout == nil {
		layout = model.JSONMap{}
	}
	_, insertErr := r.db.ExecContext(ctx,
		`INSERT INTO dashboards (group_id, created_by, name, is_default, date_range, filter, layout)
		 SELECT ?, ?, ?, 1, ?, ?, ? FROM DUAL
		 WHERE NOT EXISTS (SELECT 1 FROM dashboards WHERE group_id = ? AND is_default = 1)`,
		groupID, userID, DefaultDashboardName, DefaultDashboardDateRange, defaultFilter(), layout, groupID)
	if insertErr != nil {
		var me *mysql.MySQLError
		if !errors.As(insertErr, &me) || me.Number != mysqlDeadlock {
			logger.Error(ctx, "dashboard-repo: failed to insert default dashboard", zap.Uint64("group_id", groupID), zap.Error(insertErr))
			return nil, insertErr
		}
	}
	var d model.Dashboard
	err := r.db.GetContext(ctx, &d,
		dashboardSelect+` WHERE group_id = ? AND is_default = 1 ORDER BY id LIMIT 1`, groupID)
	if errors.Is(err, sql.ErrNoRows) && insertErr != nil {
		logger.Error(ctx, "dashboard-repo: failed to insert default dashboard", zap.Uint64("group_id", groupID), zap.Error(insertErr))
		return nil, insertErr
	}
	if err != nil {
		logger.Error(ctx, "dashboard-repo: failed to get default dashboard", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return &d, nil
}

// CreatorNames maps each existing user among userIDs to their display name,
// or their email when the display name is unset.
func (r *DashboardRepositoryImpl) CreatorNames(ctx context.Context, userIDs []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string)
	if len(userIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(
		`SELECT id, COALESCE(NULLIF(display_name, ''), email) AS name FROM users WHERE id IN (?)`, userIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID   uint64 `db:"id"`
		Name string `db:"name"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "dashboard-repo: failed to get creator names", zap.Int("users", len(userIDs)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}
