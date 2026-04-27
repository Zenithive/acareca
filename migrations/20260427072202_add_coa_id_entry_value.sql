-- +goose Up
-- +goose StatementBegin

-- Add coa_id column to support direct COA entries (for owner fund transactions)
-- This allows entries to reference COA directly without requiring a form field
ALTER TABLE tbl_form_entry_value 
ADD COLUMN IF NOT EXISTS coa_id UUID REFERENCES tbl_chart_of_accounts(id);

-- Add description column for direct entries
ALTER TABLE tbl_form_entry_value
ADD COLUMN IF NOT EXISTS description TEXT;

-- Modify constraint: either form_field_id OR coa_id must be provided (not both)
-- Note: We can't add this constraint if existing data violates it
-- So we'll add it as a comment for application-level validation
COMMENT ON COLUMN tbl_form_entry_value.coa_id IS 'Direct COA reference for entries without form fields (e.g., owner fund transactions). Either form_field_id OR coa_id must be provided, not both.';

-- Add index for performance when querying by COA
CREATE INDEX IF NOT EXISTS idx_form_entry_value_coa_id 
ON tbl_form_entry_value(coa_id) 
WHERE coa_id IS NOT NULL AND deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_form_entry_value_coa_id;
ALTER TABLE tbl_form_entry_value DROP COLUMN IF EXISTS description;
ALTER TABLE tbl_form_entry_value DROP COLUMN IF EXISTS coa_id;

-- +goose StatementEnd
