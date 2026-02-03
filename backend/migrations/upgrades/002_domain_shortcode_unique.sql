-- Migration: Change short_code uniqueness to (domain_id, short_code)
-- This allows the same short_code to exist on different domains
-- NOTE: NULL domain_id uniqueness is enforced in application layer

-- Drop the old unique constraint on short_code alone
ALTER TABLE links DROP INDEX short_code;

-- Add new composite unique constraint
-- Note: In MariaDB, NULL values in unique constraints are treated as distinct,
-- so (NULL, 'abc') can appear multiple times. App layer handles this case.
ALTER TABLE links ADD UNIQUE INDEX idx_domain_short_code (domain_id, short_code);
