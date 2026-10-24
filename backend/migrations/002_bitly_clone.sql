-- Migration 002: Bitly clone foundation (spec §6.1, §11.1)
-- Every statement is idempotent: this file may be executed more than once
-- (docker-entrypoint-initdb.d on fresh installs, then the backend runner).

-- Groups: each user owns one or more groups; links are scoped by group
CREATE TABLE IF NOT EXISTS `groups` (
    id                  BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id             BIGINT UNSIGNED NOT NULL,
    name                VARCHAR(255) NOT NULL,
    default_domain_id   BIGINT UNSIGNED NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_groups_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_groups_default_domain FOREIGN KEY (default_domain_id) REFERENCES domains(id) ON DELETE SET NULL,
    INDEX idx_groups_user_id (user_id)
);

-- Users
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS default_group_id BIGINT UNSIGNED NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS account_name VARCHAR(255) NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS home_checklist_dismissed BOOLEAN NOT NULL DEFAULT FALSE;

-- Links (group_id starts at 0, is backfilled below, then gets its foreign key)
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS group_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS has_custom_back_half BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS redirect_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS parent_link_id BIGINT UNSIGNED NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS long_url_host VARCHAR(255) NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS source ENUM('link','qr','page') NOT NULL DEFAULT 'link';

ALTER TABLE links
    ADD INDEX IF NOT EXISTS idx_links_group_created (group_id, created_at),
    ADD INDEX IF NOT EXISTS idx_links_group_archived (group_id, archived),
    ADD INDEX IF NOT EXISTS idx_links_parent (parent_link_id);

ALTER TABLE links
    ADD CONSTRAINT fk_links_parent FOREIGN KEY IF NOT EXISTS (parent_link_id) REFERENCES links(id) ON DELETE SET NULL;

-- Link tags
CREATE TABLE IF NOT EXISTS link_tags (
    link_id     BIGINT UNSIGNED NOT NULL,
    tag         VARCHAR(100) NOT NULL,
    PRIMARY KEY (link_id, tag),
    INDEX idx_link_tags_tag (tag),
    CONSTRAINT fk_link_tags_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);

-- Clicks
ALTER TABLE clicks
    ADD COLUMN IF NOT EXISTS source ENUM('link','qr','page') NOT NULL DEFAULT 'link',
    ADD COLUMN IF NOT EXISTS os VARCHAR(50) NULL DEFAULT NULL;

-- Dynamic routing rules (evaluated in position order)
CREATE TABLE IF NOT EXISTS dynamic_routing_rules (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    link_id         BIGINT UNSIGNED NOT NULL,
    position        INT NOT NULL DEFAULT 0,
    long_url        TEXT NOT NULL,
    device_match    JSON NULL,
    os_match        JSON NULL,
    country_match   JSON NULL,
    country_exclude JSON NULL,
    CONSTRAINT fk_routing_rules_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE,
    INDEX idx_routing_rules_link_position (link_id, position)
);

-- API tokens (only the SHA-256 hash is stored)
CREATE TABLE IF NOT EXISTS api_tokens (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    name            VARCHAR(255) NOT NULL,
    token_hash      CHAR(64) NOT NULL,
    token_prefix    VARCHAR(16) NOT NULL,
    last_used_at    TIMESTAMP NULL DEFAULT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked_at      TIMESTAMP NULL DEFAULT NULL,
    UNIQUE INDEX idx_api_tokens_hash (token_hash),
    INDEX idx_api_tokens_user_id (user_id),
    CONSTRAINT fk_api_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Activity log
CREATE TABLE IF NOT EXISTS activity_log (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    group_id        BIGINT UNSIGNED NOT NULL,
    action          VARCHAR(50) NOT NULL,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       BIGINT UNSIGNED NOT NULL,
    metadata        JSON NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_activity_group_created (group_id, created_at),
    CONSTRAINT fk_activity_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_activity_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);

-- Domains. verification_status is added with DEFAULT 'ok' so every domain
-- that existed before this migration is verified; new rows default to
-- 'pending' once the default is switched below (safe to re-run).
ALTER TABLE domains
    ADD COLUMN IF NOT EXISTS group_id BIGINT UNSIGNED NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS verification_token VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verification_status ENUM('pending','ok','error') NOT NULL DEFAULT 'ok',
    ADD COLUMN IF NOT EXISTS verification_error VARCHAR(512) NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMP NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMP NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS root_redirect VARCHAR(2048) NULL DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS not_found_redirect VARCHAR(2048) NULL DEFAULT NULL;

ALTER TABLE domains
    ALTER COLUMN verification_status SET DEFAULT 'pending';

ALTER TABLE domains
    ADD CONSTRAINT fk_domains_group FOREIGN KEY IF NOT EXISTS (group_id) REFERENCES `groups`(id) ON DELETE SET NULL;

-- Backfill: one group per user, named after the email local part
INSERT INTO `groups` (user_id, name)
SELECT u.id, SUBSTRING_INDEX(u.email, '@', 1)
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM `groups` g WHERE g.user_id = u.id);

UPDATE users u
SET u.default_group_id = (SELECT MIN(g.id) FROM `groups` g WHERE g.user_id = u.id)
WHERE u.default_group_id IS NULL;

UPDATE links l
JOIN users u ON u.id = l.user_id
SET l.group_id = u.default_group_id
WHERE l.group_id = 0 AND u.default_group_id IS NOT NULL;

UPDATE links
SET long_url_host = LOWER(SUBSTRING_INDEX(SUBSTRING_INDEX(SUBSTRING_INDEX(SUBSTRING_INDEX(
        SUBSTRING_INDEX(original_url, '://', -1), '/', 1), '?', 1), '#', 1), ':', 1))
WHERE long_url_host IS NULL;

UPDATE domains
SET verified_at = created_at
WHERE verification_status = 'ok' AND verified_at IS NULL;

-- ON DELETE CASCADE: groups are deleted with their user (users -> groups
-- cascade), and RESTRICT here would make user deletion fail depending on
-- cascade order.
ALTER TABLE links
    ADD CONSTRAINT fk_links_group FOREIGN KEY IF NOT EXISTS (group_id) REFERENCES `groups`(id) ON DELETE CASCADE;
