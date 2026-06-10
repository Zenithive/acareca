-- +goose Up
-- +goose StatementBegin

-- 1. Rename 'entity_id' back to 'practitioner_id'
ALTER TABLE tbl_custom_form_version RENAME COLUMN entity_id TO practitioner_id;

-- 2. Add 'entity_id' as a new column
ALTER TABLE tbl_custom_form_version ADD COLUMN IF NOT EXISTS entity_id UUID;

-- 3. Sync the data
UPDATE tbl_custom_form_version SET entity_id = practitioner_id WHERE entity_id IS NULL;

-- 4. Add index for fast lookups
CREATE INDEX IF NOT EXISTS idx_custom_form_entity_id ON tbl_custom_form_version(entity_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_custom_form_entity_id;
ALTER TABLE tbl_custom_form_version DROP COLUMN IF EXISTS entity_id;
-- +goose StatementEnd