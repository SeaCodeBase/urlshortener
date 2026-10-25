package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

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

// linkColumns is the full column list of a model.Link, qualified with the
// "l" alias so it can be used in joins.
const linkColumns = `l.id, l.user_id, l.group_id, l.short_code, l.original_url, l.long_url_host, l.title,
	l.expires_at, l.is_active, l.archived, l.has_custom_back_half, l.redirect_count, l.parent_link_id,
	l.source, l.domain_id, l.created_at, l.updated_at`

func (r *LinkRepositoryImpl) Create(ctx context.Context, link *model.Link) error {
	if link.Source == "" {
		link.Source = model.LinkSourceLink
	}
	query := `INSERT INTO links (user_id, group_id, short_code, original_url, long_url_host, title, expires_at,
			  is_active, archived, has_custom_back_half, parent_link_id, source, domain_id)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		link.UserID, link.GroupID, link.ShortCode, link.OriginalURL, link.LongURLHost, link.Title, link.ExpiresAt,
		link.IsActive, link.Archived, link.HasCustomBackHalf, link.ParentLinkID, link.Source, link.DomainID)
	if err != nil {
		// The unique index uq_links_domain_key_code (migration 004) covers
		// every domain including the default one (domain_key 0).
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
	query := `SELECT ` + linkColumns + ` FROM links l WHERE l.id = ?`
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

func (r *LinkRepositoryImpl) ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error) {
	var links []model.Link
	query := `SELECT id, user_id, group_id, short_code, original_url, title, expires_at, is_active, domain_id, created_at, updated_at
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
	query := `UPDATE links SET original_url = ?, long_url_host = ?, title = ?, expires_at = ?, is_active = ?, domain_id = ?,
			  updated_at = NOW() WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, link.OriginalURL, link.LongURLHost, link.Title, link.ExpiresAt,
		link.IsActive, link.DomainID, link.ID)
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
		// MariaDB reports 0 affected rows when nothing changed (e.g. an
		// update within the same second as the previous one), so only a
		// missing row is "not found".
		var exists int
		if err := r.db.GetContext(ctx, &exists, `SELECT COUNT(*) FROM links WHERE id = ?`, link.ID); err != nil {
			return err
		}
		if exists == 0 {
			return ErrLinkNotFound
		}
	}
	return nil
}

func (r *LinkRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to begin delete", zap.Uint64("link_id", id), zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// The FK cascade removes the link's page blocks without touching their
	// pages, so mark those pages changed first (they need a republish). The
	// page ids come from a plain read so the page rows are locked before the
	// cascade locks the blocks (lock order page, then blocks).
	var pageIDs []uint64
	if err := tx.SelectContext(ctx, &pageIDs, `SELECT DISTINCT page_id FROM page_blocks WHERE link_id = ?`, id); err != nil {
		logger.Error(ctx, "link-repo: failed to read pages of link", zap.Uint64("link_id", id), zap.Error(err))
		return err
	}
	if len(pageIDs) > 0 {
		query, args, err := sqlx.In(`UPDATE pages SET revision = revision + 1 WHERE id IN (?)`, pageIDs)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, tx.Rebind(query), args...); err != nil {
			logger.Error(ctx, "link-repo: failed to bump page revisions on delete",
				zap.Uint64("link_id", id),
				zap.Error(err),
			)
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, id)
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
	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "link-repo: failed to commit delete", zap.Uint64("link_id", id), zap.Error(err))
		return err
	}
	return nil
}

func (r *LinkRepositoryImpl) ShortCodeExistsInDomain(ctx context.Context, domainID *uint64, code string) (bool, error) {
	var count int
	var query string
	var err error

	if domainID == nil {
		query = `SELECT COUNT(*) FROM links WHERE domain_id IS NULL AND short_code = ?`
		err = r.db.GetContext(ctx, &count, query, code)
	} else {
		query = `SELECT COUNT(*) FROM links WHERE domain_id = ? AND short_code = ?`
		err = r.db.GetContext(ctx, &count, query, *domainID, code)
	}

	if err != nil {
		logger.Error(ctx, "link-repo: failed to check short code existence in domain",
			zap.String("short_code", code),
			zap.Error(err),
		)
		return false, err
	}
	return count > 0, nil
}

func (r *LinkRepositoryImpl) GetByDomainAndShortCode(ctx context.Context, domainID *uint64, code string) (*model.Link, error) {
	var link model.Link
	var query string
	var err error

	if domainID == nil {
		query = `SELECT id, user_id, group_id, short_code, original_url, title, expires_at, is_active, archived, domain_id, created_at, updated_at
				 FROM links WHERE domain_id IS NULL AND short_code = ?`
		err = r.db.GetContext(ctx, &link, query, code)
	} else {
		query = `SELECT id, user_id, group_id, short_code, original_url, title, expires_at, is_active, archived, domain_id, created_at, updated_at
				 FROM links WHERE domain_id = ? AND short_code = ?`
		err = r.db.GetContext(ctx, &link, query, *domainID, code)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		logger.Error(ctx, "link-repo: failed to get link by domain and short code",
			zap.String("short_code", code),
			zap.Error(err),
		)
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepositoryImpl) CountByDomainID(ctx context.Context, domainID uint64) (int64, error) {
	var count int64
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM links WHERE domain_id = ?`, domainID); err != nil {
		logger.Error(ctx, "link-repo: failed to count links by domain ID",
			zap.Uint64("domain_id", domainID),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
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

// LinkFilter narrows ListByGroup. Zero values apply no filter.
type LinkFilter struct {
	// Query matches title, destination URL, short code or a tag (substring,
	// LIKE wildcards are matched literally).
	Query string
	// Tags keeps links having any of the tags.
	Tags []string
	// Archived keeps hidden (true) or active (false) links; nil keeps both.
	Archived *bool
	// CustomBackHalf keeps links with (true) or without (false) a custom back-half.
	CustomBackHalf *bool
	// Expiration is "expired", "expiring" (has a future expiry), "none" or "".
	Expiration string
	// HasQR keeps links with (true) or without (false) an attached QR code.
	HasQR *bool
	// CampaignID is accepted for API compatibility and ignored for now.
	CampaignID *uint64
	// CreatedAfter is inclusive, CreatedBefore exclusive.
	CreatedAfter, CreatedBefore *time.Time
	// Sort is "created" (newest first, default) or "clicks" (most first).
	Sort string
	// IncludeAllSources also returns links owned by QR codes and pages
	// (source <> 'link'), which are hidden from the Links list (spec §11.3).
	IncludeAllSources bool
}

// Expiration filter values.
const (
	ExpirationExpired  = "expired"
	ExpirationExpiring = "expiring"
	ExpirationNone     = "none"
)

// SortClicks orders ListByGroup by total clicks.
const SortClicks = "clicks"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// linkListFrom is the FROM clause of ListByGroup/CountByGroup; qr_codes.link_id
// is unique, so the join never duplicates a link.
const linkListFrom = ` FROM links l LEFT JOIN qr_codes qc ON qc.link_id = l.id`

// where builds the WHERE clause (without the keyword) and its arguments for
// a query over linkListFrom.
func (f LinkFilter) where(groupID uint64) (string, []any) {
	conds := []string{"l.group_id = ?"}
	args := []any{groupID}
	if !f.IncludeAllSources {
		conds = append(conds, "l.source = 'link'")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + likeEscaper.Replace(q) + "%"
		conds = append(conds, `(l.title LIKE ? OR l.original_url LIKE ? OR l.short_code LIKE ?
			OR EXISTS (SELECT 1 FROM link_tags qt WHERE qt.link_id = l.id AND qt.tag LIKE ?))`)
		args = append(args, like, like, like, like)
	}
	if tags := NormalizeTags(f.Tags); len(tags) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		conds = append(conds, "EXISTS (SELECT 1 FROM link_tags ft WHERE ft.link_id = l.id AND ft.tag IN ("+placeholders+"))")
		for _, t := range tags {
			args = append(args, t)
		}
	}
	if f.Archived != nil {
		conds = append(conds, "l.archived = ?")
		args = append(args, *f.Archived)
	}
	if f.CustomBackHalf != nil {
		conds = append(conds, "l.has_custom_back_half = ?")
		args = append(args, *f.CustomBackHalf)
	}
	switch f.Expiration {
	case ExpirationExpired:
		conds = append(conds, "l.expires_at IS NOT NULL AND l.expires_at <= NOW()")
	case ExpirationExpiring:
		conds = append(conds, "l.expires_at > NOW()")
	case ExpirationNone:
		conds = append(conds, "l.expires_at IS NULL")
	}
	if f.HasQR != nil {
		if *f.HasQR {
			conds = append(conds, "qc.id IS NOT NULL")
		} else {
			conds = append(conds, "qc.id IS NULL")
		}
	}
	if f.CreatedAfter != nil {
		conds = append(conds, "l.created_at >= ?")
		args = append(args, *f.CreatedAfter)
	}
	if f.CreatedBefore != nil {
		conds = append(conds, "l.created_at < ?")
		args = append(args, *f.CreatedBefore)
	}
	return strings.Join(conds, " AND "), args
}

func (r *LinkRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64, f LinkFilter, limit, offset int) ([]model.LinkWithStats, int64, error) {
	where, args := f.where(groupID)

	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*)`+linkListFrom+` WHERE `+where, args...); err != nil {
		logger.Error(ctx, "link-repo: failed to count links by group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, 0, err
	}

	order := "l.created_at DESC, l.id DESC"
	if f.Sort == SortClicks {
		order = "total_clicks DESC, l.created_at DESC, l.id DESC"
	}
	query := `SELECT ` + linkColumns + `,
			  (SELECT COUNT(*) FROM clicks c WHERE c.link_id = l.id) AS total_clicks,
			  qc.id AS qr_code_id` + linkListFrom + ` WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	rows := []model.LinkWithStats{}
	if err := r.db.SelectContext(ctx, &rows, query, append(args, limit, offset)...); err != nil {
		logger.Error(ctx, "link-repo: failed to list links by group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return nil, 0, err
	}
	return rows, total, nil
}

// CountByGroup counts the group's links matching f (by default those shown
// in the Links list: source = 'link', hidden ones included).
func (r *LinkRepositoryImpl) CountByGroup(ctx context.Context, groupID uint64, f LinkFilter) (int64, error) {
	where, args := f.where(groupID)
	var count int64
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*)`+linkListFrom+` WHERE `+where, args...)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to count links by group",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		return 0, err
	}
	return count, nil
}

// GetByIDs returns the existing links among ids (any order; missing ids are
// simply absent).
func (r *LinkRepositoryImpl) GetByIDs(ctx context.Context, ids []uint64) ([]model.Link, error) {
	links := []model.Link{}
	if len(ids) == 0 {
		return links, nil
	}
	query, args, err := sqlx.In(`SELECT `+linkColumns+` FROM links l WHERE l.id IN (?)`, ids)
	if err != nil {
		return nil, err
	}
	if err := r.db.SelectContext(ctx, &links, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "link-repo: failed to get links by IDs",
			zap.Int("count", len(ids)),
			zap.Error(err),
		)
		return nil, err
	}
	return links, nil
}

// SetArchived hides (archived=true) or shows the links. Empty ids is a no-op.
func (r *LinkRepositoryImpl) SetArchived(ctx context.Context, ids []uint64, archived bool) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE links SET archived = ?, updated_at = NOW() WHERE id IN (?)`, archived, ids)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "link-repo: failed to set archived",
			zap.Int("count", len(ids)),
			zap.Bool("archived", archived),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// IncrementRedirectCount records a destination change ("redirect").
func (r *LinkRepositoryImpl) IncrementRedirectCount(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE links SET redirect_count = redirect_count + 1 WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "link-repo: failed to increment redirect count",
			zap.Uint64("link_id", id),
			zap.Error(err),
		)
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
