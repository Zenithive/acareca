-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_financial_settings ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_fin_settings_deleted_at ON tbl_financial_settings (deleted_at) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_fin_settings_deleted_at;
ALTER TABLE tbl_financial_settings DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd