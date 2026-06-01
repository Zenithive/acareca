-- +goose Up
-- +goose StatementBegin
CREATE TYPE account_classification AS ENUM (
    'Current Asset', 
    'Non-Current Asset', 
    'Contra-Asset', 
    'Current Liability', 
    'Non-Current Liability', 
    'Equity', 
    'Contra-Equity', 
    'Operating Revenue', 
    'Other Revenue', 
    'Direct Costs', 
    'Operating Expense'
);

ALTER TABLE tbl_chart_of_accounts 
ADD COLUMN classification account_classification;

UPDATE tbl_chart_of_accounts coa
SET classification = CASE 
    WHEN t.name = 'Asset' THEN 'Current Asset'::account_classification
    WHEN t.name = 'Liability' THEN 'Current Liability'::account_classification
    WHEN t.name = 'Equity' THEN 'Equity'::account_classification
    WHEN t.name = 'Revenue' THEN 'Operating Revenue'::account_classification
    WHEN t.name = 'Expense' THEN 'Operating Expense'::account_classification
END
FROM tbl_account_type t
WHERE coa.account_type_id = t.id;

ALTER TABLE tbl_chart_of_accounts 
ALTER COLUMN classification SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_chart_of_accounts 
DROP COLUMN IF EXISTS classification;

DROP TYPE IF EXISTS account_classification;
-- +goose StatementEnd