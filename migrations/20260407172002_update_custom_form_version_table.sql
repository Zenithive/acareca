-- +goose Up
-- +goose StatementBegin
-- 1. Rename the column in the custom form version table
ALTER TABLE tbl_custom_form_version RENAME COLUMN practitioner_id TO entity_id;

-- 2. Add an index for the new entity_id to keep form lookups fast
CREATE INDEX IF NOT EXISTS idx_custom_form_entity_id ON tbl_custom_form_version(entity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 1. Remove the index
DROP INDEX IF EXISTS idx_custom_form_entity_id;

-- 2. Rename the column back
ALTER TABLE tbl_custom_form_version RENAME COLUMN entity_id TO practitioner_id;
-- +goose StatementEnd