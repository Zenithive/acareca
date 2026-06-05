-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value 
ADD COLUMN IF NOT EXISTS business_percentage DECIMAL(5, 2) DEFAULT 100.00 
CHECK (business_percentage >= 0 AND business_percentage <= 100);

ALTER TABLE tbl_form_entry_value 
ADD COLUMN IF NOT EXISTS notes TEXT DEFAULT '-';

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_form_entry_value_business_percentage 
ON tbl_form_entry_value(business_percentage) 
WHERE business_percentage != 100.00;

COMMENT ON COLUMN tbl_form_entry_value.business_percentage IS 'Business use percentage (0-100), default 100%';
COMMENT ON COLUMN tbl_form_entry_value.notes IS 'Additional notes for the entry, default "-"';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_form_entry_value_business_percentage;

ALTER TABLE tbl_form_entry_value 
DROP COLUMN IF EXISTS notes;

ALTER TABLE tbl_form_entry_value 
DROP COLUMN IF EXISTS business_percentage;
-- +goose StatementEnd
