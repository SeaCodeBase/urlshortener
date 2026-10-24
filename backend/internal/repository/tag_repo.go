package repository

import (
	"context"
	"sort"
	"strings"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// MaxTagLength is the maximum tag length in characters (link_tags.tag).
const MaxTagLength = 100

// Compile-time check: TagRepositoryImpl implements TagRepository
var _ TagRepository = (*TagRepositoryImpl)(nil)

type TagRepositoryImpl struct {
	db *sqlx.DB
}

func NewTagRepository(db *sqlx.DB) *TagRepositoryImpl {
	return &TagRepositoryImpl{db: db}
}

// NormalizeTags trims tags, truncates them to MaxTagLength characters, drops
// blanks and removes (case-sensitive) duplicates, keeping first-seen order.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if r := []rune(t); len(r) > MaxTagLength {
			t = strings.TrimSpace(string(r[:MaxTagLength]))
		}
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// insertTags inserts every (linkID, tag) pair, ignoring pairs that already
// exist (the column collation may treat case variants as equal).
func insertTags(ctx context.Context, ex sqlx.ExecerContext, linkIDs []uint64, tags []string) error {
	if len(linkIDs) == 0 || len(tags) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("INSERT IGNORE INTO link_tags (link_id, tag) VALUES ")
	args := make([]any, 0, len(linkIDs)*len(tags)*2)
	for _, id := range linkIDs {
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

func (r *TagRepositoryImpl) SetLinkTags(ctx context.Context, linkID uint64, tags []string) error {
	tags = NormalizeTags(tags)
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM link_tags WHERE link_id = ?", linkID); err != nil {
		logger.Error(ctx, "tag-repo: failed to clear link tags", zap.Uint64("link_id", linkID), zap.Error(err))
		return err
	}
	if err := insertTags(ctx, tx, []uint64{linkID}, tags); err != nil {
		logger.Error(ctx, "tag-repo: failed to insert link tags", zap.Uint64("link_id", linkID), zap.Error(err))
		return err
	}
	return tx.Commit()
}

func (r *TagRepositoryImpl) GetLinkTags(ctx context.Context, linkID uint64) ([]string, error) {
	tags := []string{}
	if err := r.db.SelectContext(ctx, &tags, "SELECT tag FROM link_tags WHERE link_id = ? ORDER BY tag", linkID); err != nil {
		logger.Error(ctx, "tag-repo: failed to get link tags", zap.Uint64("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return tags, nil
}

func (r *TagRepositoryImpl) GetTagsForLinks(ctx context.Context, linkIDs []uint64) (map[uint64][]string, error) {
	out := make(map[uint64][]string)
	if len(linkIDs) == 0 {
		return out, nil
	}
	query, args, err := sqlx.In("SELECT link_id, tag FROM link_tags WHERE link_id IN (?) ORDER BY link_id, tag", linkIDs)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		LinkID uint64 `db:"link_id"`
		Tag    string `db:"tag"`
	}
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "tag-repo: failed to get tags for links", zap.Int("links", len(linkIDs)), zap.Error(err))
		return nil, err
	}
	for _, row := range rows {
		out[row.LinkID] = append(out[row.LinkID], row.Tag)
	}
	return out, nil
}

func (r *TagRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64) ([]string, error) {
	tags := []string{}
	query := `SELECT DISTINCT lt.tag FROM link_tags lt
		JOIN links l ON l.id = lt.link_id
		WHERE l.group_id = ?`
	if err := r.db.SelectContext(ctx, &tags, query, groupID); err != nil {
		logger.Error(ctx, "tag-repo: failed to list group tags", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	sort.Strings(tags)
	return tags, nil
}

func (r *TagRepositoryImpl) AddTagsToLinks(ctx context.Context, linkIDs []uint64, tags []string) error {
	if err := insertTags(ctx, r.db, linkIDs, NormalizeTags(tags)); err != nil {
		logger.Error(ctx, "tag-repo: failed to add tags to links", zap.Int("links", len(linkIDs)), zap.Error(err))
		return err
	}
	return nil
}

func (r *TagRepositoryImpl) RemoveTagsFromLinks(ctx context.Context, linkIDs []uint64, tags []string) error {
	tags = NormalizeTags(tags)
	if len(linkIDs) == 0 || len(tags) == 0 {
		return nil
	}
	query, args, err := sqlx.In("DELETE FROM link_tags WHERE link_id IN (?) AND tag IN (?)", linkIDs, tags)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(query), args...); err != nil {
		logger.Error(ctx, "tag-repo: failed to remove tags from links", zap.Int("links", len(linkIDs)), zap.Error(err))
		return err
	}
	return nil
}
