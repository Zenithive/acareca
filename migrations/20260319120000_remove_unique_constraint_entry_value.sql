-- +goose Up
-- +goose StatementBegin

-- 1. Remove the unique constraint to allow multiple records per field
ALTER TABLE tbl_form_entry_value 
DROP CONSTRAINT IF EXISTS tbl_form_entry_value_entry_id_form_field_id_key;

-- 2. Make updated_at nullable and remove default so we can use it for versioning
ALTER TABLE tbl_form_entry_value 
ALTER COLUMN updated_at DROP DEFAULT,
ALTER COLUMN updated_at DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 1. Restore the unique constraint (Warning: will fail if duplicates exist)
ALTER TABLE tbl_form_entry_value 
ADD CONSTRAINT tbl_form_entry_value_entry_id_form_field_id_key UNIQUE(entry_id, form_field_id);

-- 2. Restore defaults and non-null (Warning: will fail if NULLs exist)
ALTER TABLE tbl_form_entry_value 
ALTER COLUMN updated_at SET DEFAULT now(),
ALTER COLUMN updated_at SET NOT NULL;

-- +goose StatementEnd