package repository

import (
	"context"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Compile-time check: ActivityRepositoryImpl implements ActivityRepository
var _ ActivityRepository = (*ActivityRepositoryImpl)(nil)

const activityColumns = "id, user_id, group_id, action, entity_type, entity_id, metadata, created_at"

type ActivityRepositoryImpl struct {
	db *sqlx.DB
}

func NewActivityRepository(db *sqlx.DB) *ActivityRepositoryImpl {
	return &ActivityRepositoryImpl{db: db}
}

func (r *ActivityRepositoryImpl) Insert(ctx context.Context, entry *model.ActivityEntry) error {
	query := "INSERT INTO activity_log (user_id, group_id, action, entity_type, entity_id, metadata) VALUES (?, ?, ?, ?, ?, ?)"
	result, err := r.db.ExecContext(ctx, query, entry.UserID, entry.GroupID, entry.Action, entry.EntityType, entry.EntityID, entry.Metadata)
	if err != nil {
		logger.Error(ctx, "activity-repo: failed to insert entry",
			zap.Uint64("group_id", entry.GroupID),
			zap.String("action", entry.Action),
			zap.Error(err),
		)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = uint64(id)
	if err := r.db.GetContext(ctx, &entry.CreatedAt, "SELECT created_at FROM activity_log WHERE id = ?", entry.ID); err != nil {
		logger.Error(ctx, "activity-repo: failed to reload created_at", zap.Uint64("id", entry.ID), zap.Error(err))
		return err
	}
	return nil
}

func (r *ActivityRepositoryImpl) List(ctx context.Context, groupID uint64, from, to *time.Time, limit, offset int) ([]model.ActivityEntry, int64, error) {
	where := []string{"group_id = ?"}
	args := []any{groupID}
	if from != nil {
		where = append(where, "created_at >= ?")
		args = append(args, from.UTC())
	}
	if to != nil {
		where = append(where, "created_at < ?")
		args = append(args, to.UTC())
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM activity_log WHERE "+cond, args...); err != nil {
		logger.Error(ctx, "activity-repo: failed to count entries", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, 0, err
	}

	entries := []model.ActivityEntry{}
	query := "SELECT " + activityColumns + " FROM activity_log WHERE " + cond + " ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"
	if err := r.db.SelectContext(ctx, &entries, query, append(args, limit, offset)...); err != nil {
		logger.Error(ctx, "activity-repo: failed to list entries", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, 0, err
	}
	return entries, total, nil
}
