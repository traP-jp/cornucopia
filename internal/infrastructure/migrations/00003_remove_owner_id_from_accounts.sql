-- +goose Up
-- +goose StatementBegin
ALTER TABLE accounts DROP INDEX idx_owner_id;
ALTER TABLE accounts DROP COLUMN owner_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE accounts ADD COLUMN owner_id BINARY(16) NOT NULL UNIQUE AFTER id;
ALTER TABLE accounts ADD INDEX idx_owner_id (owner_id);
-- +goose StatementEnd
