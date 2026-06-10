-- +goose Up
-- +goose StatementBegin
-- Drop the unique constraint on (form_version_id, field_key)
-- This allows multiple fields to have the same key within a form version
DROP INDEX IF EXISTS uniq_form_field_key;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Recreate the unique constraint if rolling back
-- Note: This will fail if duplicate keys exist in the data
CREATE UNIQUE INDEX uniq_form_field_key
ON tbl_form_field(form_version_id, field_key)
WHERE deleted_at IS NULL;
-- +goose StatementEnd
