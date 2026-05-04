-- +goose Up
-- Remove clinic_id from BAS views since BAS preparation now works at practitioner level only
-- This simplifies the views and aligns with the new architecture

-- ============================================================
-- UPDATE BAS VIEWS - Remove clinic_id
-- ============================================================

-- +goose StatementBegin
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
-- +goose StatementEnd

-- +goose StatementBegin
-- Base view without clinic_id
CREATE VIEW vw_bas_line_items AS
SELECT
    cfv.practitioner_id,
    f.id AS form_id, 
    f.name AS form_name,
    fe.id AS entry_id, 
    fe.submitted_at,
    -- Use date if available, otherwise fall back to submitted_at
    DATE_TRUNC('month',   COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_month,
    DATE_TRUNC('quarter', COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_quarter,
    DATE_TRUNC('year',    COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_year,
    ff.id AS form_field_id, 
    ff.label AS field_label,
    ff.section_type, 
    ff.payment_responsibility, 
    ff.tax_type,
    coa.id AS coa_id, 
    coa.code AS account_code, 
    coa.name AS account_name,
    atx.id AS account_tax_id, 
    atx.name AS tax_name, 
    atx.rate AS tax_rate, 
    atx.is_taxable,
    CASE
        WHEN atx.name = 'BAS Excluded' THEN 'BAS_EXCLUDED'
        WHEN atx.is_taxable = TRUE     THEN 'TAXABLE'
        ELSE                                'GST_FREE'
    END AS bas_category,
    COALESCE(fev.net_amount,   0) AS net_amount,
    COALESCE(fev.gst_amount,   0) AS gst_amount,
    COALESCE(fev.gross_amount, 0) AS gross_amount
FROM tbl_form_entry_value    fev
JOIN tbl_form_entry          fe  ON fe.id  = fev.entry_id
JOIN tbl_form_field          ff  ON ff.id  = fev.form_field_id
JOIN tbl_custom_form_version cfv ON cfv.id = ff.form_version_id
JOIN tbl_form                f   ON f.id   = cfv.form_id
JOIN tbl_chart_of_accounts   coa ON coa.id = ff.coa_id
JOIN tbl_account_tax         atx ON atx.id = coa.account_tax_id
WHERE fe.status    = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND coa.deleted_at IS NULL
  AND ff.section_type IS NOT NULL
  AND ff.coa_id IS NOT NULL
  AND fev.updated_at IS NULL;  -- Only get active (non-updated) entry values
-- +goose StatementEnd

-- +goose StatementBegin
-- Summary view without clinic_id
CREATE OR REPLACE VIEW vw_bas_summary AS
WITH base AS (
    SELECT practitioner_id, period_month, period_quarter, period_year,
           section_type, bas_category, net_amount, gst_amount, gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    practitioner_id, period_quarter, period_year,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS g1_total_sales_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g3_gst_free_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g8_taxable_sales,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) AS label_1a_gst_on_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS g11_total_purchases_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g15_taxable_purchases,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS label_1b_gst_on_purchases,
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS total_purchases_net
FROM base
GROUP BY practitioner_id, period_quarter, period_year
ORDER BY period_year, period_quarter;
-- +goose StatementEnd

-- +goose StatementBegin
-- Monthly view without clinic_id
CREATE OR REPLACE VIEW vw_bas_monthly AS
WITH base AS (
    SELECT practitioner_id, period_month, section_type, bas_category, net_amount, gst_amount, gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    practitioner_id, period_month,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS g1_total_sales_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g3_gst_free_sales,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) AS label_1a_gst_on_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS g11_total_purchases_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS label_1b_gst_on_purchases,
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS total_purchases_net
FROM base
GROUP BY practitioner_id, period_month
ORDER BY period_month;
-- +goose StatementEnd

-- +goose StatementBegin
-- By account view without clinic_id
CREATE OR REPLACE VIEW vw_bas_by_account AS
SELECT
    practitioner_id, period_quarter, period_year,
    section_type, bas_category, account_code, account_name, tax_name, tax_rate,
    COUNT(DISTINCT entry_id) AS entry_count,
    SUM(net_amount) AS total_net, SUM(gst_amount) AS total_gst, SUM(gross_amount) AS total_gross
FROM vw_bas_line_items
WHERE bas_category != 'BAS_EXCLUDED'
GROUP BY practitioner_id, period_quarter, period_year, section_type, bas_category, account_code, account_name, tax_name, tax_rate
ORDER BY period_year, period_quarter, section_type, account_code;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restore views with clinic_id
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;

-- Recreate with clinic_id (from previous migration)
CREATE VIEW vw_bas_line_items AS
SELECT
    fe.clinic_id, 
    cfv.practitioner_id,
    f.id AS form_id, 
    f.name AS form_name,
    fe.id AS entry_id, 
    fe.submitted_at,
    DATE_TRUNC('month',   COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_month,
    DATE_TRUNC('quarter', COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_quarter,
    DATE_TRUNC('year',    COALESCE(fe.date::timestamp, fe.submitted_at)) AS period_year,
    ff.id AS form_field_id, 
    ff.label AS field_label,
    ff.section_type, 
    ff.payment_responsibility, 
    ff.tax_type,
    coa.id AS coa_id, 
    coa.code AS account_code, 
    coa.name AS account_name,
    atx.id AS account_tax_id, 
    atx.name AS tax_name, 
    atx.rate AS tax_rate, 
    atx.is_taxable,
    CASE
        WHEN atx.name = 'BAS Excluded' THEN 'BAS_EXCLUDED'
        WHEN atx.is_taxable = TRUE     THEN 'TAXABLE'
        ELSE                                'GST_FREE'
    END AS bas_category,
    COALESCE(fev.net_amount,   0) AS net_amount,
    COALESCE(fev.gst_amount,   0) AS gst_amount,
    COALESCE(fev.gross_amount, 0) AS gross_amount
FROM tbl_form_entry_value    fev
JOIN tbl_form_entry          fe  ON fe.id  = fev.entry_id
JOIN tbl_form_field          ff  ON ff.id  = fev.form_field_id
JOIN tbl_custom_form_version cfv ON cfv.id = ff.form_version_id
JOIN tbl_form                f   ON f.id   = cfv.form_id
JOIN tbl_chart_of_accounts   coa ON coa.id = ff.coa_id
JOIN tbl_account_tax         atx ON atx.id = coa.account_tax_id
WHERE fe.status    = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND coa.deleted_at IS NULL
  AND ff.section_type IS NOT NULL
  AND ff.coa_id IS NOT NULL
  AND fev.updated_at IS NULL;

CREATE OR REPLACE VIEW vw_bas_summary AS
WITH base AS (
    SELECT clinic_id, practitioner_id, period_month, period_quarter, period_year,
           section_type, bas_category, net_amount, gst_amount, gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    clinic_id, practitioner_id, period_quarter, period_year,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS g1_total_sales_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g3_gst_free_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g8_taxable_sales,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) AS label_1a_gst_on_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS g11_total_purchases_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g15_taxable_purchases,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS label_1b_gst_on_purchases,
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_quarter, period_year
ORDER BY clinic_id, period_year, period_quarter;

CREATE OR REPLACE VIEW vw_bas_monthly AS
WITH base AS (
    SELECT clinic_id, practitioner_id, period_month, section_type, bas_category, net_amount, gst_amount, gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    clinic_id, practitioner_id, period_month,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS g1_total_sales_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0) AS g3_gst_free_sales,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) AS label_1a_gst_on_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS g11_total_purchases_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS label_1b_gst_on_purchases,
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY') AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST','EXPENSE_ENTRY')), 0) AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_month
ORDER BY clinic_id, period_month;

CREATE OR REPLACE VIEW vw_bas_by_account AS
SELECT
    clinic_id, practitioner_id, period_quarter, period_year,
    section_type, bas_category, account_code, account_name, tax_name, tax_rate,
    COUNT(DISTINCT entry_id) AS entry_count,
    SUM(net_amount) AS total_net, SUM(gst_amount) AS total_gst, SUM(gross_amount) AS total_gross
FROM vw_bas_line_items
WHERE bas_category != 'BAS_EXCLUDED'
GROUP BY clinic_id, practitioner_id, period_quarter, period_year, section_type, bas_category, account_code, account_name, tax_name, tax_rate
ORDER BY clinic_id, period_year, period_quarter, section_type, account_code;
-- +goose StatementEnd
