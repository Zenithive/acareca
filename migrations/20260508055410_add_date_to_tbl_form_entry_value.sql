-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value ADD COLUMN IF NOT EXISTS "date" DATE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value DROP COLUMN IF EXISTS "date";
-- +goose StatementEnd
