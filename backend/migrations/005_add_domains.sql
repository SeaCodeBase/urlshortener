-- 005_add_domains.sql

-- Create domains table
CREATE TABLE IF NOT EXISTS domains (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    domain      VARCHAR(255) NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_domains_user_id (user_id)
);

-- Add domain_id to links table
ALTER TABLE links ADD COLUMN domain_id BIGINT UNSIGNED NULL;
ALTER TABLE links ADD CONSTRAINT fk_links_domain FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL;
ALTER TABLE links ADD INDEX idx_links_domain_id (domain_id);
