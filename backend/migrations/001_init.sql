-- Consolidated database schema for URL Shortener

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email           VARCHAR(255) NOT NULL UNIQUE,
    display_name    VARCHAR(255) DEFAULT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Domains table
CREATE TABLE IF NOT EXISTS domains (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    domain      VARCHAR(255) NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_domains_user_id (user_id)
);

-- Links table
CREATE TABLE IF NOT EXISTS links (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    domain_id       BIGINT UNSIGNED NULL,
    short_code      VARCHAR(16) NOT NULL,
    original_url    TEXT NOT NULL,
    title           VARCHAR(255),
    expires_at      TIMESTAMP NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL,
    -- Unique constraint on (domain, short_code) - NULL domain_id uniqueness handled in app layer
    UNIQUE INDEX idx_domain_short_code (domain_id, short_code),
    INDEX idx_short_code (short_code),
    INDEX idx_user_id (user_id)
);

-- Clicks table
CREATE TABLE IF NOT EXISTS clicks (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    link_id         BIGINT UNSIGNED NOT NULL,
    clicked_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_hash         VARCHAR(64),
    ip_address      VARCHAR(45),
    user_agent      VARCHAR(512),
    referrer        VARCHAR(2048),
    country         VARCHAR(2),
    city            VARCHAR(100),
    device_type     ENUM('desktop', 'mobile', 'tablet', 'unknown') DEFAULT 'unknown',
    browser         VARCHAR(50),
    utm_source      VARCHAR(255),
    utm_medium      VARCHAR(255),
    utm_campaign    VARCHAR(255),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE,
    INDEX idx_link_clicked (link_id, clicked_at),
    INDEX idx_clicks_ip_address (ip_address)
);

-- Daily stats table
CREATE TABLE IF NOT EXISTS link_stats_daily (
    link_id         BIGINT UNSIGNED NOT NULL,
    date            DATE NOT NULL,
    total_clicks    INT UNSIGNED DEFAULT 0,
    unique_visitors INT UNSIGNED DEFAULT 0,
    PRIMARY KEY (link_id, date),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);

-- Passkeys table for WebAuthn 2FA
CREATE TABLE IF NOT EXISTS passkeys (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    name            VARCHAR(255) NOT NULL,
    credential_id   VARBINARY(1024) NOT NULL,
    public_key      VARBINARY(1024) NOT NULL,
    counter         INT UNSIGNED NOT NULL DEFAULT 0,
    backup_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    backup_state    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at    TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_credential (credential_id(255))
);
