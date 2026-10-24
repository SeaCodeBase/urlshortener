-- Migration 006: analytics dashboards (spec §5.8)
-- filter and layout are JSON documents validated by the service layer. There
-- is no unique index on (group_id, is_default): it would also reject a second
-- non-default row; DashboardRepository.EnsureDefault inserts the single
-- default dashboard with INSERT ... WHERE NOT EXISTS instead.
-- Every statement is idempotent: this file may be executed more than once.

CREATE TABLE IF NOT EXISTS dashboards (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    group_id    BIGINT UNSIGNED NOT NULL,
    created_by  BIGINT UNSIGNED NOT NULL,
    name        VARCHAR(255) NOT NULL,
    is_default  TINYINT(1) NOT NULL DEFAULT 0,
    date_range  VARCHAR(20) NOT NULL DEFAULT 'last_7_days',
    filter      JSON NOT NULL,
    layout      JSON NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_dashboards_group (group_id, created_at),
    CONSTRAINT fk_dashboards_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
    CONSTRAINT fk_dashboards_user FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
);
