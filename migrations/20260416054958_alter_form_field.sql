-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field ADD COLUMN is_highlighted_field BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field DROP COLUMN IF EXISTS is_highlighted_field;
-- +goose StatementEnd
