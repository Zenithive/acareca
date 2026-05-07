-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_document
    DROP COLUMN IF EXISTS entity_type,
    DROP COLUMN IF EXISTS profile_picture,
    DROP COLUMN IF EXISTS entity_id;

ALTER TABLE tbl_clinic ADD COLUMN document_id TEXT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_document
    ADD COLUMN IF NOT EXISTS entity_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS profile_picture TEXT,
    ADD COLUMN IF NOT EXISTS entity_id UUID;
    
ALTER TABLE tbl_clinic DROP COLUMN IF EXISTS document_id;
-- +goose StatementEnd
