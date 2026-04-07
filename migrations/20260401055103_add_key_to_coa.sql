-- +goose Up
-- +goose StatementBegin

-- Add key column to tbl_chart_of_accounts
ALTER TABLE tbl_chart_of_accounts
ADD COLUMN key VARCHAR(255);

-- Generate keys for existing records based on name

-- Make key NOT NULL after populating
ALTER TABLE tbl_chart_of_accounts
ALTER COLUMN key SET NOT NULL;

-- Create unique index for key + practitioner_id (with soft delete support)
CREATE UNIQUE INDEX uq_chart_of_accounts_key_practitioner_id
ON tbl_chart_of_accounts (key, practitioner_id)
WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Drop unique index
DROP INDEX IF EXISTS uq_chart_of_accounts_key_practitioner_id;

-- Drop key column
ALTER TABLE tbl_chart_of_accounts
DROP COLUMN IF EXISTS key;

-- +goose StatementEnd
