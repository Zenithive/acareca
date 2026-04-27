-- +goose Up
-- +goose StatementBegin

-- Update existing Owner Drawings and Funds Introduced accounts from Liability (2) to Equity (3)
-- This fixes the misclassification identified in the owner fund analysis
UPDATE tbl_chart_of_accounts
SET account_type_id = 3, updated_at = NOW()
WHERE code IN (880, 881)
  AND account_type_id = 2
  AND deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert back to Liability (not recommended, but for rollback capability)
UPDATE tbl_chart_of_accounts
SET account_type_id = 2, updated_at = NOW()
WHERE code IN (880, 881)
  AND account_type_id = 3
  AND deleted_at IS NULL;

-- +goose StatementEnd
