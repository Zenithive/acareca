-- +goose Up
-- +goose StatementBegin

ALTER TABLE IF EXISTS tbl_subscription
ADD COLUMN IF NOT EXISTS is_visible BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE IF EXISTS tbl_subscription
DROP COLUMN IF EXISTS is_visible;
-- +goose StatementEnd