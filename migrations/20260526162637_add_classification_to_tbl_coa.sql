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
    WHEN coa.code IN (711, 721) THEN 'Contra-Asset'::account_classification
    WHEN coa.code IN (710, 720) THEN 'Non-Current Asset'::account_classification
    WHEN coa.code IN (600, 610, 620, 630) THEN 'Current Asset'::account_classification
    WHEN coa.code IN (900) THEN 'Non-Current Liability'::account_classification
    WHEN coa.code IN (800, 801, 804, 820, 825, 826, 830, 840, 850, 860, 877) THEN 'Current Liability'::account_classification
    WHEN coa.code IN (880) THEN 'Contra-Equity'::account_classification
    WHEN coa.code IN (881, 960, 970) THEN 'Equity'::account_classification
    WHEN coa.code IN (203) THEN 'Other Revenue'::account_classification
    WHEN coa.code IN (200, 201, 202) THEN 'Operating Revenue'::account_classification
    WHEN coa.code IN (402, 403, 414) THEN 'Direct Costs'::account_classification
    WHEN coa.code BETWEEN 400 AND 499 THEN 'Operating Expense'::account_classification
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
