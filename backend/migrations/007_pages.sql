-- Migration 007: link-in-bio pages (spec Phase 4)
-- pages.slug uses the table's case-insensitive collation; the service stores
-- slugs lowercase. published_content is the JSON snapshot served publicly
-- (NULL until the first publish). A page block referencing a link is removed
-- with the link (fk_page_blocks_link ON DELETE CASCADE).
-- Every statement is idempotent: this file may be executed more than once.

CREATE TABLE IF NOT EXISTS pages (
    id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id           BIGINT UNSIGNED NOT NULL,
    group_id          BIGINT UNSIGNED NOT NULL,
    slug              VARCHAR(40) NOT NULL,
    title             VARCHAR(255) NOT NULL DEFAULT '',
    description       VARCHAR(500) NOT NULL DEFAULT '',
    avatar_url        VARCHAR(512) NOT NULL DEFAULT '',
    design            JSON NOT NULL,
    status            ENUM('draft','published') NOT NULL DEFAULT 'draft',
    published_content JSON NULL,
    published_at      TIMESTAMP NULL,
    archived          TINYINT(1) NOT NULL DEFAULT 0,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_pages_slug (slug),
    INDEX idx_pages_group (group_id, created_at),
    CONSTRAINT fk_pages_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_pages_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS page_blocks (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    page_id    BIGINT UNSIGNED NOT NULL,
    type       ENUM('link','text','image','social') NOT NULL,
    position   INT NOT NULL DEFAULT 0,
    content    JSON NOT NULL,
    is_active  TINYINT(1) NOT NULL DEFAULT 1,
    is_pinned  TINYINT(1) NOT NULL DEFAULT 0,
    link_id    BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_page_blocks_page (page_id, position),
    INDEX idx_page_blocks_link (link_id),
    CONSTRAINT fk_page_blocks_page FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE,
    CONSTRAINT fk_page_blocks_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS page_views (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    page_id     BIGINT UNSIGNED NOT NULL,
    viewed_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_hash     VARCHAR(64) NOT NULL DEFAULT '',
    referrer    VARCHAR(512) NOT NULL DEFAULT '',
    country     VARCHAR(2) NOT NULL DEFAULT '',
    city        VARCHAR(100) NOT NULL DEFAULT '',
    device_type VARCHAR(20) NOT NULL DEFAULT '',
    browser     VARCHAR(50) NOT NULL DEFAULT '',
    INDEX idx_page_views_page_time (page_id, viewed_at),
    CONSTRAINT fk_page_views_page FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE
);
