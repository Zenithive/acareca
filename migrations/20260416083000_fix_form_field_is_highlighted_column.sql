-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field RENAME COLUMN is_highlighted_field TO is_highlighted;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field RENAME COLUMN is_highlighted TO is_highlighted_field;
-- +goose StatementEnd
