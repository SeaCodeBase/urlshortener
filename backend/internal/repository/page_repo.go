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

var (
	ErrPageNotFound  = errors.New("page not found")
	ErrBlockNotFound = errors.New("block not found")
	ErrPageSlugTaken = errors.New("page slug already taken")
	// ErrInvalidBlockOrder: ReorderBlocks ids are not exactly the page's
	// block ids.
	ErrInvalidBlockOrder = errors.New("block ids do not match the page's blocks")
)

// Compile-time check: PageRepositoryImpl implements PageRepository
var _ PageRepository = (*PageRepositoryImpl)(nil)

type PageRepositoryImpl struct {
	db *sqlx.DB
}

func NewPageRepository(db *sqlx.DB) *PageRepositoryImpl {
	return &PageRepositoryImpl{db: db}
}

const pageColumns = `p.id, p.user_id, p.group_id, p.slug, p.title, p.description, p.avatar_url, p.design,
	p.status, p.published_content, p.published_at, p.archived, p.created_at, p.updated_at`

const pageSelect = `SELECT ` + pageColumns + ` FROM pages p`

const blockSelect = `SELECT id, page_id, type, position, content, is_active, is_pinned, link_id,
	created_at, updated_at FROM page_blocks`

// pageRow scans published_content separately so SQL NULL becomes a nil map
// (model.JSONMap scans NULL to an empty map).
type pageRow struct {
	model.Page
	PublishedContentRaw []byte `db:"published_content"`
}

func (r *pageRow) page() (*model.Page, error) {
	p := r.Page
	p.PublishedContent = nil
	if r.PublishedContentRaw != nil {
		var m model.JSONMap
		if err := m.Scan(r.PublishedContentRaw); err != nil {
			return nil, err
		}
		p.PublishedContent = m
	}
	return &p, nil
}

func (r *PageRepositoryImpl) Create(ctx context.Context, p *model.Page) error {
	if p.Design == nil {
		p.Design = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO pages (user_id, group_id, slug, title, description, avatar_url, design)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.GroupID, p.Slug, p.Title, p.Description, p.AvatarURL, p.Design)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == mysqlDuplicateEntry {
			return ErrPageSlugTaken
		}
		logger.Error(ctx, "page-repo: failed to create page", zap.Uint64("group_id", p.GroupID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = uint64(id)
	var row struct {
		Status    string    `db:"status"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	if err := r.db.GetContext(ctx, &row, `SELECT status, created_at, updated_at FROM pages WHERE id = ?`, p.ID); err != nil {
		logger.Error(ctx, "page-repo: failed to read created page", zap.Uint64("page_id", p.ID), zap.Error(err))
		return err
	}
	p.Status, p.CreatedAt, p.UpdatedAt = row.Status, row.CreatedAt, row.UpdatedAt
	p.PublishedContent, p.PublishedAt, p.Archived = nil, nil, false
	return nil
}

func (r *PageRepositoryImpl) getOne(ctx context.Context, where string, arg any) (*model.Page, error) {
	var row pageRow
	err := r.db.GetContext(ctx, &row, pageSelect+` WHERE `+where, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPageNotFound
	}
	if err != nil {
		logger.Error(ctx, "page-repo: failed to get page", zap.Any("key", arg), zap.Error(err))
		return nil, err
	}
	return row.page()
}

func (r *PageRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Page, error) {
	return r.getOne(ctx, `p.id = ?`, id)
}

// GetBySlug matches case-insensitively (the slug column's collation).
func (r *PageRepositoryImpl) GetBySlug(ctx context.Context, slug string) (*model.Page, error) {
	return r.getOne(ctx, `p.slug = ?`, slug)
}

func (r *PageRepositoryImpl) SlugExists(ctx context.Context, slug string) (bool, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM pages WHERE slug = ?`, slug); err != nil {
		logger.Error(ctx, "page-repo: failed to check slug", zap.String("slug", slug), zap.Error(err))
		return false, err
	}
	return n > 0, nil
}

func (r *PageRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64, f PageFilter) ([]PageWithViews, error) {
	query := `SELECT ` + pageColumns + `, COALESCE(v.views, 0) AS views FROM pages p
		LEFT JOIN (SELECT page_id, COUNT(*) AS views FROM page_views GROUP BY page_id) v ON v.page_id = p.id
		WHERE p.group_id = ? AND p.archived = ?`
	args := []any{groupID, f.Archived}
	if f.Query != "" {
		like := "%" + likeEscaper.Replace(f.Query) + "%"
		query += ` AND (p.title LIKE ? OR p.slug LIKE ?)`
		args = append(args, like, like)
	}
	query += ` ORDER BY p.created_at DESC, p.id DESC`
	var rows []struct {
		pageRow
		Views int64 `db:"views"`
	}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		logger.Error(ctx, "page-repo: failed to list pages", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	out := make([]PageWithViews, 0, len(rows))
	for i := range rows {
		p, err := rows[i].page()
		if err != nil {
			return nil, err
		}
		out = append(out, PageWithViews{Page: *p, Views: rows[i].Views})
	}
	return out, nil
}

// requireAffected maps 0 affected rows to notFound unless the row exists (an
// UPDATE that changes nothing also affects 0 rows).
func (r *PageRepositoryImpl) requireAffected(ctx context.Context, result sql.Result, table string, id uint64, notFound error) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var exists int
	if err := r.db.GetContext(ctx, &exists, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id); err != nil {
		return err
	}
	if exists == 0 {
		return notFound
	}
	return nil
}

func (r *PageRepositoryImpl) Update(ctx context.Context, p *model.Page) error {
	if p.Design == nil {
		p.Design = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE pages SET title = ?, description = ?, avatar_url = ?, design = ? WHERE id = ?`,
		p.Title, p.Description, p.AvatarURL, p.Design, p.ID)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to update page", zap.Uint64("page_id", p.ID), zap.Error(err))
		return err
	}
	return r.requireAffected(ctx, result, "pages", p.ID, ErrPageNotFound)
}

func (r *PageRepositoryImpl) SetArchived(ctx context.Context, id uint64, archived bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE pages SET archived = ?, updated_at = updated_at WHERE id = ?`, archived, id)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to set archived", zap.Uint64("page_id", id), zap.Error(err))
		return err
	}
	return r.requireAffected(ctx, result, "pages", id, ErrPageNotFound)
}

func (r *PageRepositoryImpl) Publish(ctx context.Context, id uint64, content model.JSONMap, at time.Time) error {
	if content == nil {
		content = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE pages SET status = ?, published_content = ?, published_at = ?, updated_at = updated_at WHERE id = ?`,
		model.PageStatusPublished, content, at.UTC(), id)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to publish page", zap.Uint64("page_id", id), zap.Error(err))
		return err
	}
	return r.requireAffected(ctx, result, "pages", id, ErrPageNotFound)
}

func (r *PageRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to delete page", zap.Uint64("page_id", id), zap.Error(err))
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPageNotFound
	}
	return nil
}

func (r *PageRepositoryImpl) ListBlocks(ctx context.Context, pageID uint64) ([]model.PageBlock, error) {
	rows := []model.PageBlock{}
	if err := r.db.SelectContext(ctx, &rows, blockSelect+` WHERE page_id = ? ORDER BY position, id`, pageID); err != nil {
		logger.Error(ctx, "page-repo: failed to list blocks", zap.Uint64("page_id", pageID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *PageRepositoryImpl) GetBlock(ctx context.Context, id uint64) (*model.PageBlock, error) {
	var b model.PageBlock
	err := r.db.GetContext(ctx, &b, blockSelect+` WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBlockNotFound
	}
	if err != nil {
		logger.Error(ctx, "page-repo: failed to get block", zap.Uint64("block_id", id), zap.Error(err))
		return nil, err
	}
	return &b, nil
}

// CreateBlock computes the position in the INSERT itself so concurrent
// appends never read a stale maximum.
func (r *PageRepositoryImpl) CreateBlock(ctx context.Context, b *model.PageBlock) error {
	if b.Content == nil {
		b.Content = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO page_blocks (page_id, type, position, content, is_active, is_pinned, link_id)
		 SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?, ?, ? FROM page_blocks WHERE page_id = ?`,
		b.PageID, b.Type, b.Content, b.IsActive, b.IsPinned, b.LinkID, b.PageID)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to create block", zap.Uint64("page_id", b.PageID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	created, err := r.GetBlock(ctx, uint64(id))
	if err != nil {
		return err
	}
	*b = *created
	return nil
}

func (r *PageRepositoryImpl) UpdateBlock(ctx context.Context, b *model.PageBlock) error {
	if b.Content == nil {
		b.Content = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE page_blocks SET content = ?, is_active = ?, is_pinned = ? WHERE id = ?`,
		b.Content, b.IsActive, b.IsPinned, b.ID)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to update block", zap.Uint64("block_id", b.ID), zap.Error(err))
		return err
	}
	return r.requireAffected(ctx, result, "page_blocks", b.ID, ErrBlockNotFound)
}

func (r *PageRepositoryImpl) DeleteBlock(ctx context.Context, id uint64) error {
	// Bump the page's updated_at first so the deletion counts as an
	// unpublished change (the block row is gone afterwards).
	if _, err := r.db.ExecContext(ctx,
		`UPDATE pages SET updated_at = CURRENT_TIMESTAMP WHERE id = (SELECT page_id FROM page_blocks WHERE id = ?)`, id); err != nil {
		logger.Error(ctx, "page-repo: failed to touch page for block delete", zap.Uint64("block_id", id), zap.Error(err))
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM page_blocks WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to delete block", zap.Uint64("block_id", id), zap.Error(err))
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrBlockNotFound
	}
	return nil
}

func (r *PageRepositoryImpl) ReorderBlocks(ctx context.Context, pageID uint64, ids []uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "page-repo: failed to begin reorder", zap.Uint64("page_id", pageID), zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current []uint64
	if err := tx.SelectContext(ctx, &current, `SELECT id FROM page_blocks WHERE page_id = ? FOR UPDATE`, pageID); err != nil {
		logger.Error(ctx, "page-repo: failed to lock blocks", zap.Uint64("page_id", pageID), zap.Error(err))
		return err
	}
	if len(ids) != len(current) {
		return ErrInvalidBlockOrder
	}
	remaining := make(map[uint64]bool, len(current))
	for _, id := range current {
		remaining[id] = true
	}
	for _, id := range ids {
		if !remaining[id] {
			return ErrInvalidBlockOrder // foreign, unknown or duplicate id
		}
		delete(remaining, id)
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE page_blocks SET position = ? WHERE id = ? AND page_id = ?`, i, id, pageID); err != nil {
			logger.Error(ctx, "page-repo: failed to reorder block", zap.Uint64("block_id", id), zap.Error(err))
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "page-repo: failed to commit reorder", zap.Uint64("page_id", pageID), zap.Error(err))
		return err
	}
	return nil
}

func (r *PageRepositoryImpl) PagesForLinks(ctx context.Context, linkIDs []uint64) (map[uint64][]uint64, error) {
	out := make(map[uint64][]uint64)
	if len(linkIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(
		`SELECT DISTINCT b.link_id, b.page_id FROM page_blocks b
		 JOIN pages p ON p.id = b.page_id
		 WHERE b.link_id IN (?) AND p.archived = 0
		 ORDER BY b.link_id, b.page_id`, linkIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		LinkID uint64 `db:"link_id"`
		PageID uint64 `db:"page_id"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "page-repo: failed to get pages for links", zap.Int("links", len(linkIDs)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.LinkID] = append(out[row.LinkID], row.PageID)
	}
	return out, nil
}

func (r *PageRepositoryImpl) MaxBlockUpdatedAt(ctx context.Context, pageID uint64) (*time.Time, error) {
	var at sql.NullTime
	if err := r.db.GetContext(ctx, &at, `SELECT MAX(updated_at) FROM page_blocks WHERE page_id = ?`, pageID); err != nil {
		logger.Error(ctx, "page-repo: failed to get max block updated_at", zap.Uint64("page_id", pageID), zap.Error(err))
		return nil, err
	}
	if !at.Valid {
		return nil, nil
	}
	t := at.Time
	return &t, nil
}
