-- Migration 005: QR codes (spec §6.1, §11.3)
-- A qr_codes row always owns exactly one link (link_id UNIQUE); deleting the
-- link deletes the QR code. There is no expires_at column: QR expiration
-- lives on the owned link (links.expires_at), which is what the redirect
-- enforces (ruling: one source of truth, although spec §6.1 listed one).
-- Every statement is idempotent: this file may be executed more than once.

CREATE TABLE IF NOT EXISTS qr_codes (
    id                      BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id                 BIGINT UNSIGNED NOT NULL,
    group_id                BIGINT UNSIGNED NOT NULL,
    link_id                 BIGINT UNSIGNED NOT NULL,
    title                   VARCHAR(255) NULL DEFAULT NULL,
    render_customizations   JSON NOT NULL,
    archived                TINYINT(1) NOT NULL DEFAULT 0,
    created_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX uq_qr_codes_link (link_id),
    INDEX idx_qr_codes_group_created (group_id, created_at),
    INDEX idx_qr_codes_group_archived (group_id, archived),
    CONSTRAINT fk_qr_codes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_qr_codes_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
    CONSTRAINT fk_qr_codes_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);

-- QR code tags
CREATE TABLE IF NOT EXISTS qr_tags (
    qr_code_id  BIGINT UNSIGNED NOT NULL,
    tag         VARCHAR(100) NOT NULL,
    PRIMARY KEY (qr_code_id, tag),
    INDEX idx_qr_tags_tag (tag),
    CONSTRAINT fk_qr_tags_qr_code FOREIGN KEY (qr_code_id) REFERENCES qr_codes(id) ON DELETE CASCADE
);
