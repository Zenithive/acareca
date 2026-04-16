-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value 
ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;

CREATE INDEX idx_form_entry_value_deleted_at 
ON tbl_form_entry_value (deleted_at) 
WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_form_entry_value_deleted_at;

ALTER TABLE tbl_form_entry_value 
DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd