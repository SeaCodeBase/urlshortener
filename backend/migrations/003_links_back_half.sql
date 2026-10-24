-- Migration 003: custom back-halves (spec §5.3)
-- Back-halves are 1-255 characters and case-sensitive, so short_code is
-- widened and given a binary collation (the (domain_id, short_code) unique
-- index then treats "AbC" and "abc" as different codes). Idempotent.
ALTER TABLE links
    MODIFY COLUMN short_code VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;
