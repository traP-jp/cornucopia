-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transactions (
    -- UUIDv7: time-ordered, can ORDER BY id DESC for recent-first queries
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
    INDEX idx_idempotency_key (idempotency_key)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS transactions;
-- +goose StatementEnd
