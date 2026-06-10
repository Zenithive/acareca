-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_clinic 
    ALTER COLUMN name TYPE VARCHAR(150);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_clinic 
    ALTER COLUMN name TYPE TEXT;
-- +goose StatementEnd