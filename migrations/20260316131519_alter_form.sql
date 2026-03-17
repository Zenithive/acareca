-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field
    ALTER COLUMN payment_responsibility DROP NOT NULL,
    ALTER COLUMN tax_type DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field
    ALTER COLUMN payment_responsibility SET NOT NULL,
    ALTER COLUMN tax_type SET NOT NULL;
-- +goose StatementEnd
