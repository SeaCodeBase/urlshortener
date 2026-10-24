-- Migration 004: one short-code namespace per domain, including the default
-- domain. A UNIQUE index on (domain_id, short_code) does not stop duplicate
-- default-domain codes because NULLs never collide, so links get a stored
-- generated domain_key (0 = default domain) and the unique index moves to
-- (domain_key, short_code). Idempotent.
--
-- MariaDB does not allow a generated column over a column whose foreign key
-- has ON DELETE SET NULL, and SET NULL silently moved a deleted domain's
-- links into the default namespace anyway. The domain foreign key (unnamed
-- in 001, so links_ibfk_2) becomes fk_links_domain with ON DELETE RESTRICT:
-- the API refuses to delete a domain that still has links.
ALTER TABLE links
    DROP FOREIGN KEY IF EXISTS links_ibfk_2;

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS domain_key BIGINT UNSIGNED AS (COALESCE(domain_id, 0)) STORED;

-- domain_id needs an index of its own once the old composite unique index
-- (which also served the foreign key) is gone.
ALTER TABLE links
    ADD INDEX IF NOT EXISTS idx_links_domain_id (domain_id);

ALTER TABLE links
    ADD UNIQUE INDEX IF NOT EXISTS uq_links_domain_key_code (domain_key, short_code);

ALTER TABLE links
    DROP INDEX IF EXISTS idx_domain_short_code;

ALTER TABLE links
    ADD CONSTRAINT fk_links_domain FOREIGN KEY IF NOT EXISTS (domain_id) REFERENCES domains(id) ON DELETE RESTRICT;
