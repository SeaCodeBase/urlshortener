-- Migration: Change short_code uniqueness to (domain_id, short_code)
-- This allows the same short_code to exist on different domains

-- Add generated column that treats NULL domain_id as 0 for uniqueness
ALTER TABLE links ADD COLUMN domain_id_key BIGINT UNSIGNED
    GENERATED ALWAYS AS (COALESCE(domain_id, 0)) STORED;

-- Drop the old unique constraint on short_code alone
ALTER TABLE links DROP INDEX short_code;

-- Add new composite unique constraint
ALTER TABLE links ADD UNIQUE INDEX idx_domain_short_code (domain_id_key, short_code);
