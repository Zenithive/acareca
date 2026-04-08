-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field
    ADD COLUMN IF NOT EXISTS is_formula BOOLEAN DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field
    DROP COLUMN IF EXISTS is_formula;
-- +goose StatementEnd
