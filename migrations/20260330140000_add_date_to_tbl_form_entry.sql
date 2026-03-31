-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry
    ADD COLUMN IF NOT EXISTS date DATE NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_entry
    DROP COLUMN IF EXISTS date;
-- +goose StatementEnd
