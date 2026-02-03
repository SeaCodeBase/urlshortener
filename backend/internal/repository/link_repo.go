package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrLinkNotFound = errors.New("link not found")
var ErrShortCodeExists = errors.New("short code already exists")

// Compile-time check: LinkRepositoryImpl implements LinkRepository
var _ LinkRepository = (*LinkRepositoryImpl)(nil)

type LinkRepositoryImpl struct {
	db *sqlx.DB
}

func NewLinkRepository(db *sqlx.DB) *LinkRepositoryImpl {
	return &LinkRepositoryImpl{db: db}
}

func (r *LinkRepositoryImpl) Create(ctx context.Context, link *model.Link) error {
	query := `INSERT INTO links (user_id, short_code, original_url, title, expires_at, is_active, domain_id)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		link.UserID, link.ShortCode, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.DomainID)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrShortCodeExists
		}
		logger.Error(ctx, "link-repo: failed to create link",
			zap.Uint64("user_id", link.UserID),
			zap.Error(err),
		)
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get last insert ID",
			zap.Error(err),
		)
		return err
	}
	link.ID = uint64(id)

	// Fetch the created record to get DB-generated timestamps
	created, err := r.GetByID(ctx, link.ID)
	if err != nil {
		return err
	}
	link.CreatedAt = created.CreatedAt
	link.UpdatedAt = created.UpdatedAt

	return nil
}

func (r *LinkRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, domain_id, created_at, updated_at
			  FROM links WHERE id = ?`
	err := r.db.GetContext(ctx, &link, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get link by ID",
			zap.Uint64("link_id", id),
			zap.Error(err),
		)
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepositoryImpl) GetByShortCode(ctx context.Context, code string) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, domain_id, created_at, updated_at
			  FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &link, query, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get link by short code",
			zap.String("short_code", code),
			zap.Error(err),
		)
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepositoryImpl) ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error) {
	var links []model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, domain_id, created_at, updated_at
			  FROM links WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	err := r.db.SelectContext(ctx, &links, query, userID, limit, offset)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to list links by user ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return links, nil
}

func (r *LinkRepositoryImpl) Update(ctx context.Context, link *model.Link) error {
	query := `UPDATE links SET original_url = ?, title = ?, expires_at = ?, is_active = ?, domain_id = ?, updated_at = NOW()
			  WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.DomainID, link.ID)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to update link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get rows affected",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		return err
	}
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

func (r *LinkRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM links WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to delete link",
			zap.Uint64("link_id", id),
			zap.Error(err),
		)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get rows affected on delete",
			zap.Uint64("link_id", id),
			zap.Error(err),
		)
		return err
	}
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

func (r *LinkRepositoryImpl) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &count, query, code)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to check short code existence",
			zap.String("short_code", code),
			zap.Error(err),
		)
		return false, err
	}
	return count > 0, nil
}

func (r *LinkRepositoryImpl) CountByUserID(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM links WHERE user_id = ?`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to count links by user ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
}
