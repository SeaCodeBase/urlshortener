-- Migration 008: page revision counter (Phase 4 fix wave)
-- revision is bumped by every page or block write (and by deleting a link a
-- block references); publish copies the revision it snapshotted into
-- published_revision. has_unpublished_changes is
-- status <> 'published' OR revision <> published_revision.
-- The UPDATE backfills pages published before this migration: those whose
-- page and blocks were not edited after published_at start clean; the rest
-- keep published_revision 0 and show unpublished changes.
-- Every statement is idempotent: this file may be executed more than once.

ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS published_revision BIGINT UNSIGNED NOT NULL DEFAULT 0;

UPDATE pages p
SET p.published_revision = p.revision, p.updated_at = p.updated_at
WHERE p.status = 'published'
  AND p.published_revision = 0
  AND p.published_at IS NOT NULL
  AND p.updated_at <= p.published_at
  AND NOT EXISTS (SELECT 1 FROM page_blocks b WHERE b.page_id = p.id AND b.updated_at > p.published_at);
