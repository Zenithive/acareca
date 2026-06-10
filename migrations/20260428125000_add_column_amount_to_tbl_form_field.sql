-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field 
ADD COLUMN IF NOT EXISTS amount DOUBLE PRECISION NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field 
DROP COLUMN IF EXISTS amount;
-- +goose StatementEnd