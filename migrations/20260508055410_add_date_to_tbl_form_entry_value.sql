-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value ADD COLUMN IF NOT EXISTS date DATE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
