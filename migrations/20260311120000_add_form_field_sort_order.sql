-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_tbl_form_field_sort_order ON tbl_form_field(form_version_id, sort_order);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tbl_form_field_sort_order;
ALTER TABLE tbl_form_field DROP COLUMN IF EXISTS sort_order;
-- +goose StatementEnd
