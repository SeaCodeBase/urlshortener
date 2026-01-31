-- Create passkeys table for WebAuthn 2FA
CREATE TABLE passkeys (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    credential_id VARBINARY(1024) NOT NULL,
    public_key VARBINARY(1024) NOT NULL,
    counter INT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_credential (credential_id)
);
