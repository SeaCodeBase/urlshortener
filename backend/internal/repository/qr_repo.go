package repository

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrQRCodeNotFound = errors.New("qr code not found")
var ErrQRCodeExistsForLink = errors.New("link already has a qr code")

// mysqlDuplicateEntry is MySQL/MariaDB error ER_DUP_ENTRY.
const mysqlDuplicateEntry = 1062

// SortScans orders QRRepository.ListByGroup by total scans.
const SortScans = "scans"

// Compile-time check: QRRepositoryImpl implements QRRepository
var _ QRRepository = (*QRRepositoryImpl)(nil)

type QRRepositoryImpl struct {
	db *sqlx.DB
}

func NewQRRepository(db *sqlx.DB) *QRRepositoryImpl {
	return &QRRepositoryImpl{db: db}
}

// QRFilter narrows QRRepository.ListByGroup. Zero values apply no filter.
type QRFilter struct {
	// Query matches the QR title, the link's title, destination URL, short
	// code or a QR tag (substring, LIKE wildcards are matched literally).
	Query string
	// Tags keeps QR codes having any of the tags.
	Tags []string
	// Archived keeps hidden (true) or active (false) QR codes; nil keeps both.
	Archived *bool
	// Expiration ("expired", "expiring", "none" or "") applies to the owned
	// link's expires_at.
	Expiration string
	// HasLink keeps QR codes whose link is also a Links-list link
	// (source = 'link', true) or owned by the QR code only (false).
	HasLink *bool
	// CreatedAfter is inclusive, CreatedBefore exclusive (QR creation time).
	CreatedAfter, CreatedBefore *time.Time
	// Sort is "created" (newest first, default) or "scans" (most first).
	Sort string
}

// QRCodeRow is a QR code with the fields of its link and its scan count
// (clicks with source = 'qr').
type QRCodeRow struct {
	model.QRCode
	LinkUserID            uint64         `db:"link_user_id"`
	LinkShortCode         string         `db:"link_short_code"`
	LinkOriginalURL       string         `db:"link_original_url"`
	LinkLongURLHost       *string        `db:"link_long_url_host"`
	LinkTitle             *string        `db:"link_title"`
	LinkDomainID          *uint64        `db:"link_domain_id"`
	LinkSource            string         `db:"link_source"`
	LinkExpiresAt         model.NullTime `db:"link_expires_at"`
	LinkArchived          bool           `db:"link_archived"`
	LinkRedirectCount     int            `db:"link_redirect_count"`
	LinkHasCustomBackHalf bool           `db:"link_has_custom_back_half"`
	LinkIsActive          bool           `db:"link_is_active"`
	LinkParentLinkID      *uint64        `db:"link_parent_link_id"`
	LinkCreatedAt         time.Time      `db:"link_created_at"`
	LinkUpdatedAt         time.Time      `db:"link_updated_at"`
	TotalScans            int64          `db:"total_scans"`
}

// ToLink returns the owned link built from the row's link fields.
func (r QRCodeRow) ToLink() model.Link {
	return model.Link{
		ID:                r.LinkID,
		UserID:            r.LinkUserID,
		GroupID:           r.GroupID,
		ShortCode:         r.LinkShortCode,
		OriginalURL:       r.LinkOriginalURL,
		LongURLHost:       r.LinkLongURLHost,
		Title:             r.LinkTitle,
		ExpiresAt:         r.LinkExpiresAt,
		Archived:          r.LinkArchived,
		HasCustomBackHalf: r.LinkHasCustomBackHalf,
		RedirectCount:     r.LinkRedirectCount,
		Source:            r.LinkSource,
		DomainID:          r.LinkDomainID,
		IsActive:          r.LinkIsActive,
		ParentLinkID:      r.LinkParentLinkID,
		CreatedAt:         r.LinkCreatedAt,
		UpdatedAt:         r.LinkUpdatedAt,
	}
}

const qrRowSelect = `SELECT q.id, q.user_id, q.group_id, q.link_id, q.title, q.render_customizations,
	q.archived, q.created_at, q.updated_at,
	l.user_id AS link_user_id, l.short_code AS link_short_code, l.original_url AS link_original_url,
	l.long_url_host AS link_long_url_host, l.title AS link_title, l.domain_id AS link_domain_id,
	l.source AS link_source, l.expires_at AS link_expires_at, l.archived AS link_archived,
	l.redirect_count AS link_redirect_count, l.has_custom_back_half AS link_has_custom_back_half,
	l.is_active AS link_is_active, l.parent_link_id AS link_parent_link_id,
	l.created_at AS link_created_at, l.updated_at AS link_updated_at,
	(SELECT COUNT(*) FROM clicks c WHERE c.link_id = l.id AND c.source = 'qr') AS total_scans
	FROM qr_codes q JOIN links l ON l.id = q.link_id`

func (r *QRRepositoryImpl) Create(ctx context.Context, qr *model.QRCode) error {
	if qr.RenderCustomizations == nil {
		qr.RenderCustomizations = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO qr_codes (user_id, group_id, link_id, title, render_customizations, archived)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		qr.UserID, qr.GroupID, qr.LinkID, qr.Title, qr.RenderCustomizations, qr.Archived)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == mysqlDuplicateEntry {
			return ErrQRCodeExistsForLink
		}
		logger.Error(ctx, "qr-repo: failed to create qr code", zap.Uint64("link_id", qr.LinkID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	qr.ID = uint64(id)
	var ts struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	if err := r.db.GetContext(ctx, &ts, `SELECT created_at, updated_at FROM qr_codes WHERE id = ?`, qr.ID); err != nil {
		logger.Error(ctx, "qr-repo: failed to read created qr code", zap.Uint64("qr_code_id", qr.ID), zap.Error(err))
		return err
	}
	qr.CreatedAt, qr.UpdatedAt = ts.CreatedAt, ts.UpdatedAt
	return nil
}

func (r *QRRepositoryImpl) getOne(ctx context.Context, where string, arg uint64) (*QRCodeRow, error) {
	var row QRCodeRow
	err := r.db.GetContext(ctx, &row, qrRowSelect+` WHERE `+where, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQRCodeNotFound
	}
	if err != nil {
		logger.Error(ctx, "qr-repo: failed to get qr code", zap.String("where", where), zap.Uint64("arg", arg), zap.Error(err))
		return nil, err
	}
	return &row, nil
}

func (r *QRRepositoryImpl) GetByID(ctx context.Context, id uint64) (*QRCodeRow, error) {
	return r.getOne(ctx, "q.id = ?", id)
}

func (r *QRRepositoryImpl) GetByLinkID(ctx context.Context, linkID uint64) (*QRCodeRow, error) {
	return r.getOne(ctx, "q.link_id = ?", linkID)
}

func (r *QRRepositoryImpl) GetByIDs(ctx context.Context, ids []uint64) ([]QRCodeRow, error) {
	rows := []QRCodeRow{}
	if len(ids) == 0 {
		return rows, nil
	}
	query, args, err := sqlx.In(qrRowSelect+` WHERE q.id IN (?)`, ids)
	if err != nil {
		return nil, err
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "qr-repo: failed to get qr codes by IDs", zap.Int("count", len(ids)), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

// where builds the WHERE clause (without the keyword) and its arguments for
// a query over "qr_codes q JOIN links l".
func (f QRFilter) where(groupID uint64) (string, []any) {
	conds := []string{"q.group_id = ?"}
	args := []any{groupID}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + likeEscaper.Replace(q) + "%"
		conds = append(conds, `(q.title LIKE ? OR l.title LIKE ? OR l.original_url LIKE ? OR l.short_code LIKE ?
			OR EXISTS (SELECT 1 FROM qr_tags qt WHERE qt.qr_code_id = q.id AND qt.tag LIKE ?))`)
		args = append(args, like, like, like, like, like)
	}
	if tags := NormalizeTags(f.Tags); len(tags) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		conds = append(conds, "EXISTS (SELECT 1 FROM qr_tags ft WHERE ft.qr_code_id = q.id AND ft.tag IN ("+placeholders+"))")
		for _, t := range tags {
			args = append(args, t)
		}
	}
	if f.Archived != nil {
		conds = append(conds, "q.archived = ?")
		args = append(args, *f.Archived)
	}
	switch f.Expiration {
	case ExpirationExpired:
		conds = append(conds, "l.expires_at IS NOT NULL AND l.expires_at <= NOW()")
	case ExpirationExpiring:
		conds = append(conds, "l.expires_at > NOW()")
	case ExpirationNone:
		conds = append(conds, "l.expires_at IS NULL")
	}
	if f.HasLink != nil {
		if *f.HasLink {
			conds = append(conds, "l.source = 'link'")
		} else {
			conds = append(conds, "l.source <> 'link'")
		}
	}
	if f.CreatedAfter != nil {
		conds = append(conds, "q.created_at >= ?")
		args = append(args, *f.CreatedAfter)
	}
	if f.CreatedBefore != nil {
		conds = append(conds, "q.created_at < ?")
		args = append(args, *f.CreatedBefore)
	}
	return strings.Join(conds, " AND "), args
}

func (r *QRRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64, f QRFilter, limit, offset int) ([]QRCodeRow, int64, error) {
	total, err := r.CountByGroup(ctx, groupID, f)
	if err != nil {
		return nil, 0, err
	}
	where, args := f.where(groupID)
	order := "q.created_at DESC, q.id DESC"
	if f.Sort == SortScans {
		order = "total_scans DESC, q.created_at DESC, q.id DESC"
	}
	rows := []QRCodeRow{}
	query := qrRowSelect + ` WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &rows, query, append(args, limit, offset)...); err != nil {
		logger.Error(ctx, "qr-repo: failed to list qr codes by group", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *QRRepositoryImpl) CountByGroup(ctx context.Context, groupID uint64, f QRFilter) (int64, error) {
	where, args := f.where(groupID)
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM qr_codes q JOIN links l ON l.id = q.link_id WHERE `+where, args...)
	if err != nil {
		logger.Error(ctx, "qr-repo: failed to count qr codes by group", zap.Uint64("group_id", groupID), zap.Error(err))
		return 0, err
	}
	return count, nil
}

func (r *QRRepositoryImpl) Update(ctx context.Context, qr *model.QRCode) error {
	if qr.RenderCustomizations == nil {
		qr.RenderCustomizations = model.JSONMap{}
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE qr_codes SET title = ?, render_customizations = ? WHERE id = ?`,
		qr.Title, qr.RenderCustomizations, qr.ID)
	if err != nil {
		logger.Error(ctx, "qr-repo: failed to update qr code", zap.Uint64("qr_code_id", qr.ID), zap.Error(err))
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
		if err := r.db.GetContext(ctx, &exists, `SELECT COUNT(*) FROM qr_codes WHERE id = ?`, qr.ID); err != nil {
			return err
		}
		if exists == 0 {
			return ErrQRCodeNotFound
		}
	}
	return nil
}

func (r *QRRepositoryImpl) SetArchived(ctx context.Context, ids []uint64, archived bool) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE qr_codes SET archived = ? WHERE id IN (?)`, archived, ids)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "qr-repo: failed to set archived", zap.Int("count", len(ids)), zap.Bool("archived", archived), zap.Error(err))
		return err
	}
	return nil
}

func (r *QRRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM qr_codes WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "qr-repo: failed to delete qr code", zap.Uint64("qr_code_id", id), zap.Error(err))
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrQRCodeNotFound
	}
	return nil
}

func (r *QRRepositoryImpl) QRCodeIDsForLinks(ctx context.Context, linkIDs []uint64) (map[uint64]uint64, error) {
	out := make(map[uint64]uint64)
	if len(linkIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In(`SELECT link_id, id FROM qr_codes WHERE link_id IN (?)`, linkIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		LinkID uint64 `db:"link_id"`
		ID     uint64 `db:"id"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "qr-repo: failed to get qr code IDs for links", zap.Int("links", len(linkIDs)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.LinkID] = row.ID
	}
	return out, nil
}

// insertQRTags inserts every (qrID, tag) pair, ignoring pairs that already
// exist (the column collation may treat case variants as equal).
func insertQRTags(ctx context.Context, ex sqlx.ExecerContext, ids []uint64, tags []string) error {
	if len(ids) == 0 || len(tags) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("INSERT IGNORE INTO qr_tags (qr_code_id, tag) VALUES ")
	args := make([]any, 0, len(ids)*len(tags)*2)
	for _, id := range ids {
		for _, t := range tags {
			if len(args) > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(?, ?)")
			args = append(args, id, t)
		}
	}
	_, err := ex.ExecContext(ctx, sb.String(), args...)
	return err
}

func (r *QRRepositoryImpl) SetTags(ctx context.Context, qrID uint64, tags []string) error {
	tags = NormalizeTags(tags)
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM qr_tags WHERE qr_code_id = ?", qrID); err != nil {
		logger.Error(ctx, "qr-repo: failed to clear qr tags", zap.Uint64("qr_code_id", qrID), zap.Error(err))
		return err
	}
	if err := insertQRTags(ctx, tx, []uint64{qrID}, tags); err != nil {
		logger.Error(ctx, "qr-repo: failed to insert qr tags", zap.Uint64("qr_code_id", qrID), zap.Error(err))
		return err
	}
	return tx.Commit()
}

func (r *QRRepositoryImpl) GetTagsFor(ctx context.Context, ids []uint64) (map[uint64][]string, error) {
	out := make(map[uint64][]string)
	if len(ids) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In("SELECT qr_code_id, tag FROM qr_tags WHERE qr_code_id IN (?) ORDER BY qr_code_id, tag", ids)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		QRCodeID uint64 `db:"qr_code_id"`
		Tag      string `db:"tag"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "qr-repo: failed to get tags for qr codes", zap.Int("qr_codes", len(ids)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.QRCodeID] = append(out[row.QRCodeID], row.Tag)
	}
	return out, nil
}

func (r *QRRepositoryImpl) ListTagsByGroup(ctx context.Context, groupID uint64) ([]string, error) {
	tags := []string{}
	query := `SELECT DISTINCT t.tag FROM qr_tags t
		JOIN qr_codes q ON q.id = t.qr_code_id
		WHERE q.group_id = ?`
	if err := r.db.SelectContext(ctx, &tags, query, groupID); err != nil {
		logger.Error(ctx, "qr-repo: failed to list group qr tags", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	sort.Strings(tags)
	return tags, nil
}

func (r *QRRepositoryImpl) AddTags(ctx context.Context, ids []uint64, tags []string) error {
	if err := insertQRTags(ctx, r.db, ids, NormalizeTags(tags)); err != nil {
		logger.Error(ctx, "qr-repo: failed to add tags to qr codes", zap.Int("qr_codes", len(ids)), zap.Error(err))
		return err
	}
	return nil
}

func (r *QRRepositoryImpl) RemoveTags(ctx context.Context, ids []uint64, tags []string) error {
	tags = NormalizeTags(tags)
	if len(ids) == 0 || len(tags) == 0 {
		return nil
	}
	query, args, err := sqlx.In("DELETE FROM qr_tags WHERE qr_code_id IN (?) AND tag IN (?)", ids, tags)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "qr-repo: failed to remove tags from qr codes", zap.Int("qr_codes", len(ids)), zap.Error(err))
		return err
	}
	return nil
}
