-- +goose Up

-- ENUM changes (outside transaction)
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_WITHDRAWAL';
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_CONTRIBUTION';

-- +goose StatementBegin

UPDATE tbl_chart_of_accounts
SET account_type_id = 3, updated_at = NOW()
WHERE code IN (880, 881)
  AND account_type_id = 2
  AND deleted_at IS NULL;

ALTER TABLE tbl_form_entry_value 
ADD COLUMN IF NOT EXISTS coa_id UUID;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT fk_form_entry_value_coa
FOREIGN KEY (coa_id) REFERENCES tbl_chart_of_accounts(id);

CREATE INDEX IF NOT EXISTS idx_form_entry_value_coa_id 
ON tbl_form_entry_value(coa_id);

-- +goose StatementEnd


-- +goose Down
-- NOTE: ENUM values cannot be removed

-- +goose StatementBegin

UPDATE tbl_chart_of_accounts
SET account_type_id = 2, updated_at = NOW()
WHERE code IN (880, 881)
  AND account_type_id = 3
  AND deleted_at IS NULL;

DROP INDEX IF EXISTS idx_form_entry_value_coa_id;

ALTER TABLE tbl_form_entry_value 
DROP CONSTRAINT IF EXISTS fk_form_entry_value_coa;

ALTER TABLE tbl_form_entry_value 
DROP COLUMN IF EXISTS coa_id;

-- +goose StatementEnd