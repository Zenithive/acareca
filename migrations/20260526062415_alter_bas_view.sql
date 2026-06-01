-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;


CREATE OR REPLACE VIEW vw_bas_line_items AS
SELECT
    fe.clinic_id,
    cfv.practitioner_id,
    f.id                                        AS form_id,
    f.name                                      AS form_name,
    fe.id                                       AS entry_id,
    fe.submitted_at,
    DATE_TRUNC('month',   fe.submitted_at)      AS period_month,
    DATE_TRUNC('quarter', fe.submitted_at)      AS period_quarter,
    DATE_TRUNC('year',    fe.submitted_at)      AS period_year,
    ff.id                                       AS form_field_id,
    ff.label                                    AS field_label,
    ff.section_type,
    ff.payment_responsibility,
    ff.tax_type,
    coa.id                                      AS coa_id,
    coa.code                                    AS account_code,
    coa.name                                    AS account_name,
    at.name                                     AS account_type,
    atx.id                                      AS account_tax_id,
    atx.name                                    AS tax_name,
    atx.rate                                    AS tax_rate,
    atx.is_taxable,
    CASE
        WHEN atx.name = 'BAS Excluded'  THEN 'BAS_EXCLUDED'
        WHEN atx.is_taxable = TRUE       THEN 'TAXABLE'
        ELSE                                  'GST_FREE'
    END                                         AS bas_category,
    COALESCE(fev.net_amount,   0)               AS net_amount,
    COALESCE(fev.gst_amount,   0)               AS gst_amount,
    COALESCE(fev.gross_amount, 0)               AS gross_amount
FROM tbl_form_entry fe
JOIN tbl_custom_form_version cfv  ON cfv.id  = fe.form_version_id
JOIN tbl_form                f    ON f.id    = cfv.form_id
JOIN (
    SELECT DISTINCT ON (entry_id, form_field_id)
        id, entry_id, form_field_id, net_amount, gst_amount, gross_amount
    FROM tbl_form_entry_value
    WHERE updated_at IS NULL
    ORDER BY entry_id, form_field_id, created_at DESC
) fev ON fev.entry_id = fe.id
JOIN tbl_form_field          ff   ON ff.id   = fev.form_field_id
JOIN tbl_chart_of_accounts   coa  ON coa.id  = ff.coa_id
JOIN tbl_account_type        at   ON at.id   = coa.account_type_id
JOIN tbl_account_tax         atx  ON atx.id  = coa.account_tax_id
WHERE fe.status      = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND ff.is_formula  = FALSE 
  AND coa.deleted_at IS NULL;


CREATE VIEW vw_bas_summary AS
WITH base AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_quarter,
        period_year,
        account_type,
        bas_category,
        net_amount,
        gst_amount,
        gross_amount
    FROM vw_bas_line_items
    WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,
    -- G1: TOTAL SALES (all Revenue accounts)
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Revenue'), 0)::numeric, 2) AS g1_total_sales_gross,
    -- 1A: GST ON SALES (Revenue with GST)
    ROUND(COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Revenue' AND bas_category = 'TAXABLE'), 0)::numeric, 2) AS label_1a_gst_on_sales,
    -- 1B: GST ON PURCHASES (all Expense accounts with GST)
    ROUND(COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Expense' AND bas_category = 'TAXABLE'), 0)::numeric, 2) AS label_1b_gst_on_purchases,
    -- G11: TOTAL PURCHASES (all Expense accounts)
    ROUND(COALESCE(SUM(gross_amount) FILTER (WHERE account_type = 'Expense'), 0)::numeric, 2) AS g11_total_purchases_gross,
    -- NET GST PAYABLE (1A - 1B)
    ROUND((COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Revenue' AND bas_category = 'TAXABLE'), 0)
        - COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Expense' AND bas_category = 'TAXABLE'), 0))::numeric, 2) AS net_gst_payable,
    -- TOTALS FOR RECONCILIATION
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Revenue'), 0)::numeric, 2) AS total_sales_net,
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Expense'), 0)::numeric, 2) AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_quarter, period_year;

-- Recreate vw_bas_by_account with account type
CREATE OR REPLACE VIEW vw_bas_by_account AS
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,
    account_type,
    bas_category,
    account_code,
    account_name,
    tax_name,
    tax_rate,
    COUNT(DISTINCT entry_id) AS entry_count,
    SUM(net_amount) AS total_net,
    SUM(gst_amount) AS total_gst,
    SUM(gross_amount) AS total_gross
FROM vw_bas_line_items
WHERE bas_category != 'BAS_EXCLUDED'
GROUP BY
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,
    account_type,
    bas_category,
    account_code,
    account_name,
    tax_name,
    tax_rate
ORDER BY
    clinic_id,
    period_year,
    period_quarter,
    account_type,
    account_code;

-- Recreate vw_bas_monthly with account type
CREATE OR REPLACE VIEW vw_bas_monthly AS
WITH base AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_month,
        account_type,
        bas_category,
        net_amount,
        gst_amount,
        gross_amount
    FROM vw_bas_line_items
    WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    clinic_id,
    practitioner_id,
    period_month,
    -- G1: Total Sales (Revenue)
    COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Revenue'), 0) AS g1_total_sales_gross,
    -- G3: GST Free Sales
    COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Revenue' AND bas_category = 'GST_FREE'), 0) AS g3_gst_free_sales,
    -- 1A: GST on Sales
    COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Revenue' AND bas_category = 'TAXABLE'), 0) AS label_1a_gst_on_sales,
    -- G11: Total Purchases (Expenses)
    COALESCE(SUM(gross_amount) FILTER (WHERE account_type = 'Expense'), 0) AS g11_total_purchases_gross,
    -- G14: GST Free Purchases
    COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Expense' AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    -- 1B: GST on Purchases
    COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Expense' AND bas_category = 'TAXABLE'), 0) AS label_1b_gst_on_purchases,
    -- Net GST Payable
    COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Revenue' AND bas_category = 'TAXABLE'), 0)
  - COALESCE(SUM(gst_amount) FILTER (WHERE account_type = 'Expense' AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    -- Totals
    COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Revenue'), 0) AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Expense'), 0) AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_month
ORDER BY clinic_id, period_month;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
-- +goose StatementEnd
