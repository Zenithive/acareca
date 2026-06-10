-- +goose Up
-- +goose StatementBegin

-- Drop old unique constraint
ALTER TABLE tbl_chart_of_accounts
DROP CONSTRAINT IF EXISTS uq_chart_of_accounts_code_practitioner_id;

-- Create partial unique index for soft delete support
CREATE UNIQUE INDEX uq_chart_of_accounts_code_practitioner_id
ON tbl_chart_of_accounts (code, practitioner_id)
WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Remove partial unique index
DROP INDEX IF EXISTS uq_chart_of_accounts_code_practitioner_id;

-- Recreate original unique constraint
ALTER TABLE tbl_chart_of_accounts
ADD CONSTRAINT uq_chart_of_accounts_code_practitioner_id
UNIQUE (code, practitioner_id);

-- +goose StatementEnd