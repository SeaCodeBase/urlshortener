package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/SeaCodeBase/urlshortener/internal/model"
)

var ErrLinkNotFound = errors.New("link not found")
var ErrShortCodeExists = errors.New("short code already exists")

type LinkRepository struct {
	db *sqlx.DB
}

func NewLinkRepository(db *sqlx.DB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Create(ctx context.Context, link *model.Link) error {
	query := `INSERT INTO links (user_id, short_code, original_url, title, expires_at, is_active)
			  VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		link.UserID, link.ShortCode, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrShortCodeExists
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	link.ID = uint64(id)
	return nil
}

func (r *LinkRepository) GetByID(ctx context.Context, id uint64) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE id = ?`
	err := r.db.GetContext(ctx, &link, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepository) GetByShortCode(ctx context.Context, code string) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &link, query, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepository) ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error) {
	var links []model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	err := r.db.SelectContext(ctx, &links, query, userID, limit, offset)
	return links, err
}

func (r *LinkRepository) Update(ctx context.Context, link *model.Link) error {
	query := `UPDATE links SET original_url = ?, title = ?, expires_at = ?, is_active = ?, updated_at = NOW()
			  WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

func (r *LinkRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM links WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

func (r *LinkRepository) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &count, query, code)
	return count > 0, err
}

func (r *LinkRepository) CountByUserID(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM links WHERE user_id = ?`
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}
