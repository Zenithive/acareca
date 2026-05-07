-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_clinic ADD COLUMN document_id TEXT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_clinic DROP COLUMN IF EXISTS document_id;
-- +goose StatementEnd
