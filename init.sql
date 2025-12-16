-- Initialize database schema for cornucopia
-- This script is run automatically when the MariaDB container starts

CREATE TABLE IF NOT EXISTS accounts (
    id BINARY(16) PRIMARY KEY,
    owner_id BINARY(16) NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0,
    can_overdraft BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_owner_id (owner_id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id BINARY(16) PRIMARY KEY,
    from_account_id BINARY(16) NOT NULL,
    to_account_id BINARY(16) NOT NULL,
    amount BIGINT NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    prev_hash VARCHAR(64) NOT NULL DEFAULT '',
    hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_from_account (from_account_id),
    INDEX idx_to_account (to_account_id),
    INDEX idx_idempotency_key (idempotency_key),
    INDEX idx_created_at (created_at)
);
