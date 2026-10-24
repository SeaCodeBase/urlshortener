package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrGroupNotFound = errors.New("group not found")

// Compile-time check: GroupRepositoryImpl implements GroupRepository
var _ GroupRepository = (*GroupRepositoryImpl)(nil)

const groupColumns = "id, user_id, name, default_domain_id, created_at, updated_at"

type GroupRepositoryImpl struct {
	db *sqlx.DB
}

func NewGroupRepository(db *sqlx.DB) *GroupRepositoryImpl {
	return &GroupRepositoryImpl{db: db}
}

func (r *GroupRepositoryImpl) Create(ctx context.Context, group *model.Group) error {
	query := "INSERT INTO `groups` (user_id, name, default_domain_id) VALUES (?, ?, ?)"
	result, err := r.db.ExecContext(ctx, query, group.UserID, group.Name, group.DefaultDomainID)
	if err != nil {
		logger.Error(ctx, "group-repo: failed to create group",
			zap.Uint64("user_id", group.UserID),
			zap.Error(err),
		)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error(ctx, "group-repo: failed to get last insert ID",
			zap.Error(err),
		)
		return err
	}
	group.ID = uint64(id)

	// Fetch the created record to get DB-generated timestamps
	created, err := r.GetByID(ctx, group.ID)
	if err != nil {
		return err
	}
	group.CreatedAt = created.CreatedAt
	group.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *GroupRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Group, error) {
	var group model.Group
	query := "SELECT " + groupColumns + " FROM `groups` WHERE id = ?"
	err := r.db.GetContext(ctx, &group, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		logger.Error(ctx, "group-repo: failed to get group by ID",
			zap.Uint64("group_id", id),
			zap.Error(err),
		)
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepositoryImpl) ListByUserID(ctx context.Context, userID uint64) ([]model.Group, error) {
	groups := []model.Group{}
	query := "SELECT " + groupColumns + " FROM `groups` WHERE user_id = ? ORDER BY id"
	if err := r.db.SelectContext(ctx, &groups, query, userID); err != nil {
		logger.Error(ctx, "group-repo: failed to list groups by user ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return groups, nil
}

func (r *GroupRepositoryImpl) Update(ctx context.Context, group *model.Group) error {
	query := "UPDATE `groups` SET name = ?, default_domain_id = ?, updated_at = NOW() WHERE id = ?"
	result, err := r.db.ExecContext(ctx, query, group.Name, group.DefaultDomainID, group.ID)
	if err != nil {
		logger.Error(ctx, "group-repo: failed to update group",
			zap.Uint64("group_id", group.ID),
			zap.Error(err),
		)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "group-repo: failed to get rows affected",
			zap.Uint64("group_id", group.ID),
			zap.Error(err),
		)
		return err
	}
	if rows > 0 {
		return nil
	}
	// MariaDB reports 0 affected rows when nothing changed; distinguish that
	// from a missing row.
	if _, err := r.GetByID(ctx, group.ID); err != nil {
		return err
	}
	return nil
}

func (r *GroupRepositoryImpl) CountByUserID(ctx context.Context, userID uint64) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM `groups` WHERE user_id = ?"
	if err := r.db.GetContext(ctx, &count, query, userID); err != nil {
		logger.Error(ctx, "group-repo: failed to count groups by user ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
}

func (r *GroupRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM `groups` WHERE id = ?", id)
	if err != nil {
		logger.Error(ctx, "group-repo: failed to delete group",
			zap.Uint64("group_id", id),
			zap.Error(err),
		)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "group-repo: failed to get rows affected on delete",
			zap.Uint64("group_id", id),
			zap.Error(err),
		)
		return err
	}
	if rows == 0 {
		return ErrGroupNotFound
	}
	return nil
}
