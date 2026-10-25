-- Migration 009: campaigns, webhooks, bulk uploads (spec Phase 5)
-- Channels are group-scoped and shared by campaigns; campaign_links rows
-- are removed with their link, with the campaign, or when their channel is
-- removed from the campaign (fk_cl_cc ON DELETE CASCADE).
-- webhook_deliveries keeps the newest 50 rows per webhook (pruned by the
-- repository). bulk_uploads stores the uploaded CSV for the worker.
-- Every statement is idempotent: this file may be executed more than once.

CREATE TABLE IF NOT EXISTS campaigns (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    group_id    BIGINT UNSIGNED NOT NULL,
    name        VARCHAR(120) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    auto_utm    TINYINT(1) NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_campaigns_group (group_id, created_at),
    CONSTRAINT fk_campaigns_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS channels (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    group_id   BIGINT UNSIGNED NOT NULL,
    name       VARCHAR(80) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_channels_group_name (group_id, name),
    CONSTRAINT fk_channels_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS campaign_channels (
    campaign_id BIGINT UNSIGNED NOT NULL,
    channel_id  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (campaign_id, channel_id),
    CONSTRAINT fk_cc_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE,
    CONSTRAINT fk_cc_channel  FOREIGN KEY (channel_id)  REFERENCES channels(id)  ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS campaign_links (
    campaign_id BIGINT UNSIGNED NOT NULL,
    channel_id  BIGINT UNSIGNED NOT NULL,
    link_id     BIGINT UNSIGNED NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (campaign_id, channel_id, link_id),
    INDEX idx_campaign_links_link (link_id),
    CONSTRAINT fk_cl_cc   FOREIGN KEY (campaign_id, channel_id) REFERENCES campaign_channels(campaign_id, channel_id) ON DELETE CASCADE,
    CONSTRAINT fk_cl_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webhooks (
    id                   BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id              BIGINT UNSIGNED NOT NULL,
    group_id             BIGINT UNSIGNED NOT NULL,
    name                 VARCHAR(120) NOT NULL,
    url                  VARCHAR(2048) NOT NULL,
    event                VARCHAR(50) NOT NULL DEFAULT 'engagement',
    secret               VARCHAR(64) NOT NULL,
    is_active            TINYINT(1) NOT NULL DEFAULT 1,
    status               ENUM('active','alert','deactivated') NOT NULL DEFAULT 'active',
    consecutive_failures INT NOT NULL DEFAULT 0,
    failing_since        DATETIME NULL,
    last_delivery_at     DATETIME NULL,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_webhooks_group (group_id),
    CONSTRAINT fk_webhooks_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id           BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    webhook_id   BIGINT UNSIGNED NOT NULL,
    event_id     VARCHAR(64) NOT NULL,
    status_code  INT NOT NULL DEFAULT 0,
    attempt      INT NOT NULL DEFAULT 1,
    delivered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    error        VARCHAR(500) NOT NULL DEFAULT '',
    INDEX idx_deliveries_webhook (webhook_id, delivered_at),
    CONSTRAINT fk_deliveries_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bulk_uploads (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id    BIGINT UNSIGNED NOT NULL,
    group_id   BIGINT UNSIGNED NOT NULL,
    filename   VARCHAR(255) NOT NULL,
    status     ENUM('queued','processing','done','failed') NOT NULL DEFAULT 'queued',
    total      INT NOT NULL DEFAULT 0,
    processed  INT NOT NULL DEFAULT 0,
    succeeded  INT NOT NULL DEFAULT 0,
    failed     INT NOT NULL DEFAULT 0,
    source_csv MEDIUMTEXT NOT NULL,
    result_csv MEDIUMTEXT NULL,
    error      VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_bulk_group (group_id, created_at),
    INDEX idx_bulk_status (status, created_at),
    CONSTRAINT fk_bulk_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
