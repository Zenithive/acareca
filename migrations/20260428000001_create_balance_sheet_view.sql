-- +goose Up
-- +goose StatementBegin

-- Drop view if it exists (in case previous migration partially succeeded)
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;

-- ============================================================
-- VIEW: vw_balance_sheet_line_items
-- Foundation view for Balance Sheet reporting
--
-- Purpose: Raw line items from submitted form entries for balance sheet accounts
-- Key Features:
--   - Filters to Asset, Liability, and Equity accounts only
--   - Handles equity withdrawals (drawings) as negative amounts
--   - Handles equity contributions as positive amounts
--   - Preserves period aggregations for historical balance sheets
-- ============================================================
CREATE OR REPLACE VIEW vw_balance_sheet_line_items AS
SELECT
    fe.clinic_id,
    cfv.practitioner_id,
    fe.id                                       AS entry_id,
    fe.submitted_at,
    fe.date                                     AS entry_date,
    DATE_TRUNC('month', fe.submitted_at)        AS period_month,
    DATE_TRUNC('year', fe.submitted_at)         AS period_year,
    ff.id                                       AS form_field_id,
    ff.section_type,
    coa.id                                      AS coa_id,
    coa.code                                    AS account_code,
    coa.name                                    AS account_name,
    at.id                                       AS account_type_id,
    at.name                                     AS account_type,
    COALESCE(fev.net_amount, 0)                 AS net_amount,
    COALESCE(fev.gross_amount, 0)               AS gross_amount,
    
    -- Signed amounts for balance sheet calculations
    -- Assets: positive (debit balance)
    -- Liabilities: positive (credit balance)
    -- Equity contributions: positive (credit balance)
    -- Equity withdrawals (drawings): negative (debit balance, reduces equity)
    CASE 
        WHEN at.name = 'Asset' THEN COALESCE(fev.net_amount, 0)
        WHEN at.name = 'Liability' THEN COALESCE(fev.net_amount, 0)
        WHEN ff.section_type = 'EQUITY_CONTRIBUTION' THEN COALESCE(fev.net_amount, 0)
        WHEN ff.section_type = 'EQUITY_WITHDRAWAL' THEN -COALESCE(fev.net_amount, 0)
        ELSE COALESCE(fev.net_amount, 0)
    END AS signed_amount

FROM tbl_form_entry fe
JOIN tbl_custom_form_version cfv ON cfv.id = fe.form_version_id
JOIN (
    SELECT DISTINCT ON (entry_id, form_field_id)
        id, entry_id, form_field_id, net_amount, gst_amount, gross_amount
    FROM tbl_form_entry_value
    WHERE updated_at IS NULL
    ORDER BY entry_id, form_field_id, created_at DESC
) fev ON fev.entry_id = fe.id
JOIN tbl_form_field ff ON ff.id = fev.form_field_id
JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id
JOIN tbl_account_type at ON at.id = coa.account_type_id

WHERE fe.status = 'SUBMITTED'
  AND fe.deleted_at IS NULL
  AND ff.deleted_at IS NULL
  AND ff.is_formula = FALSE
  AND coa.deleted_at IS NULL
  AND at.name IN ('Asset', 'Liability', 'Equity');  -- Only balance sheet accounts

-- +goose StatementEnd

-- +goose StatementBegin

-- ============================================================
-- VIEW: vw_balance_sheet_summary
-- Balance Sheet summary by account
--
-- Purpose: Aggregated balances for each COA account
-- Usage: Primary view for balance sheet reporting
-- ============================================================
CREATE OR REPLACE VIEW vw_balance_sheet_summary AS
SELECT
    practitioner_id,
    clinic_id,
    account_type,
    account_code,
    account_name,
    coa_id,
    SUM(signed_amount) AS balance,
    COUNT(DISTINCT entry_id) AS entry_count,
    MAX(submitted_at) AS last_transaction_date
FROM vw_balance_sheet_line_items
GROUP BY practitioner_id, clinic_id, account_type, account_code, account_name, coa_id
ORDER BY account_type, account_code;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
-- +goose StatementEnd
