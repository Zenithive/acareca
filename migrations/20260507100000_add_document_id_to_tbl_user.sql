-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_user
    DROP COLUMN IF EXISTS avatar_url;

ALTER TABLE tbl_user ADD COLUMN IF NOT EXISTS document_id UUID NULL REFERENCES tbl_document(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_document
    ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(50);

ALTER TABLE tbl_user DROP COLUMN IF EXISTS document_id;
-- +goose StatementEnd
