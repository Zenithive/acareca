-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- CONSOLIDATED VIEW MIGRATION - FINAL OPTIMIZED VERSION
-- Created: 2026-04-02
-- 
-- This migration consolidates all SQL views from previous migrations
-- into their latest, optimized versions. It includes:
--   - BAS (Business Activity Statement) views
--   - P&L (Profit & Loss) views
--   - Supporting functions for date-range queries
--
-- Dependencies (in order):
--   1. vw_bas_line_items (foundation view)
--   2. vw_bas_summary, vw_bas_by_account, vw_bas_monthly
--   3. vw_pl_line_items (depends on vw_bas_line_items)
--   4. vw_pl_by_account, vw_pl_summary_monthly, etc.
--   5. Functions: fn_pl_date_range, fn_pl_summary_date_range
-- ============================================================

-- Drop all existing views and functions in reverse dependency order
DROP FUNCTION IF EXISTS fn_pl_summary_date_range(UUID, DATE, DATE);
DROP FUNCTION IF EXISTS fn_pl_date_range(UUID, DATE, DATE);
DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;
DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;
DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;
DROP VIEW IF EXISTS vw_pl_by_account CASCADE;
DROP VIEW IF EXISTS vw_pl_line_items CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_bas_line_items
-- Foundation view for all BAS and P&L reporting
--
-- Purpose: Raw line items from submitted form entries with tax metadata
-- Key Features:
--   - Deduplicates entry values using DISTINCT ON
--   - Categorizes transactions as TAXABLE, GST_FREE, or BAS_EXCLUDED
--   - Includes period aggregations (month, quarter, year)
--   - Preserves all amounts as positive values
-- ============================================================
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
JOIN tbl_account_tax         atx  ON atx.id  = coa.account_tax_id
WHERE fe.status      = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND coa.deleted_at IS NULL;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_bas_summary
-- Quarterly BAS summary per clinic (mirrors ATO BAS form)
--
-- Purpose: Calculate GST obligations for Business Activity Statement
-- Key Calculations:
--   - G1: Total sales (gross revenue from patients)
--   - 1A: GST collected on sales
--   - 1B: GST credits on purchases (all costs and other costs)
--   - G11: Total purchases (all costs and other costs)
--   - Net GST: 1A - 1B (payable to ATO or refund)
-- ============================================================
CREATE VIEW vw_bas_summary AS
WITH base AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_quarter,
        period_year,
        section_type,
        bas_category,
        net_amount,
        gst_amount,
        gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,
    -- G1: TOTAL SALES
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)::numeric, 2) AS g1_total_sales_gross,
    -- 1A: GST ON SALES
    ROUND(COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)::numeric, 2) AS label_1a_gst_on_sales,
    -- 1B: GST ON PURCHASES
    ROUND(COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)::numeric, 2) AS label_1b_gst_on_purchases,
    -- G11: TOTAL PURCHASES
    ROUND(COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)::numeric, 2) AS g11_total_purchases_gross,
    -- NET GST PAYABLE (1A - 1B)
    ROUND((COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)
        - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0))::numeric, 2) AS net_gst_payable,
    -- TOTALS FOR RECONCILIATION
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)::numeric, 2) AS total_sales_net,
    ROUND(COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)::numeric, 2) AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_quarter, period_year;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_bas_by_account
-- BAS breakdown per COA account per quarter
--
-- Purpose: Detailed view showing which accounts drive GST figures
-- Usage: Drill-down analysis for BAS reporting
-- ============================================================
CREATE OR REPLACE VIEW vw_bas_by_account AS
SELECT clinic_id, practitioner_id, period_quarter, period_year,
       section_type, bas_category, account_code, account_name, tax_name, tax_rate,
       COUNT(DISTINCT entry_id) AS entry_count,
       SUM(net_amount) AS total_net, 
       SUM(gst_amount) AS total_gst, 
       SUM(gross_amount) AS total_gross
FROM vw_bas_line_items
WHERE bas_category != 'BAS_EXCLUDED'
GROUP BY clinic_id, practitioner_id, period_quarter, period_year,
         section_type, bas_category, account_code, account_name, tax_name, tax_rate
ORDER BY clinic_id, period_year, period_quarter, section_type, account_code;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_bas_monthly
-- Monthly BAS summary (same as quarterly but by month)
--
-- Purpose: Monthly accrual tracking and dashboard reporting
-- Note: Uses traditional BAS calculation (not service fee model)
-- ============================================================
CREATE OR REPLACE VIEW vw_bas_monthly AS
WITH base AS (
    SELECT clinic_id, practitioner_id, period_month, section_type, bas_category,
           net_amount, gst_amount, gross_amount
    FROM vw_bas_line_items WHERE bas_category != 'BAS_EXCLUDED'
)
SELECT clinic_id, practitioner_id, period_month,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)                                   AS g1_total_sales_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0)     AS g3_gst_free_sales,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)      AS label_1a_gst_on_sales,
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST','OTHER_COST')), 0)                         AS g11_total_purchases_gross,
    COALESCE(SUM(net_amount)   FILTER (WHERE section_type IN ('COST','OTHER_COST') AND bas_category = 'GST_FREE'), 0) AS g14_gst_free_purchases,
    COALESCE(SUM(gst_amount)   FILTER (WHERE section_type IN ('COST','OTHER_COST') AND bas_category = 'TAXABLE'), 0)  AS label_1b_gst_on_purchases,
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)
  - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST','OTHER_COST') AND bas_category = 'TAXABLE'), 0) AS net_gst_payable,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)                                     AS total_sales_net,
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST','OTHER_COST')), 0)                           AS total_purchases_net
FROM base
GROUP BY clinic_id, practitioner_id, period_month
ORDER BY clinic_id, period_month;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_line_items
-- Foundation view for Profit & Loss reporting
--
-- Purpose: Line items filtered for P&L (Revenue/Expense accounts only)
-- Key Features:
--   - Built on top of vw_bas_line_items for consistency
--   - Adds signed amounts (positive for income, negative for expenses)
--   - Categorizes into P&L sections (Income, COGS, Other Expenses)
--   - Filters to Revenue and Expense account types only
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_line_items AS
SELECT
    bli.clinic_id, bli.practitioner_id, bli.form_id, bli.form_name,
    bli.entry_id, bli.submitted_at, bli.period_month, bli.period_quarter, bli.period_year,
    bli.form_field_id, bli.field_label, bli.section_type, bli.payment_responsibility, bli.tax_type,
    bli.coa_id, bli.account_code, bli.account_name,
    at.name AS account_type,
    bli.tax_name, bli.tax_rate, bli.is_taxable,
    bli.net_amount, bli.gst_amount, bli.gross_amount,
    CASE WHEN bli.section_type = 'COLLECTION' THEN  bli.net_amount   ELSE -bli.net_amount   END AS signed_net_amount,
    CASE WHEN bli.section_type = 'COLLECTION' THEN  bli.gross_amount ELSE -bli.gross_amount END AS signed_gross_amount,
    CASE bli.section_type
        WHEN 'COLLECTION' THEN '1. Income'
        WHEN 'COST'       THEN '2. Cost of Sales'
        WHEN 'OTHER_COST' THEN '3. Other Expenses'
    END AS pl_section
FROM vw_bas_line_items bli
JOIN tbl_chart_of_accounts coa ON coa.id = bli.coa_id AND coa.deleted_at IS NULL
JOIN tbl_account_type      at  ON at.id  = coa.account_type_id
WHERE at.name IN ('Revenue', 'Expense');

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_by_account
-- P&L totals per COA account grouped by month
--
-- Purpose: Detailed account-level P&L analysis
-- Usage: Filter by practitioner_id and period_month in queries
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_by_account AS
SELECT practitioner_id, period_month, pl_section, section_type,
       account_code, account_name, account_type, tax_name, tax_rate,
       SUM(net_amount) AS total_net, 
       SUM(gst_amount) AS total_gst, 
       SUM(gross_amount) AS total_gross,
       SUM(signed_net_amount) AS signed_net, 
       SUM(signed_gross_amount) AS signed_gross,
       COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY practitioner_id, period_month, pl_section, section_type,
         account_code, account_name, account_type, tax_name, tax_rate
ORDER BY period_month, pl_section, account_code;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_summary_monthly
-- Monthly P&L summary (Income → COGS → Gross Profit → Net Profit)
--
-- Purpose: High-level monthly P&L statement
-- Key Metrics:
--   - Income (net, gst, gross)
--   - COGS (net, gst, gross)
--   - Gross Profit = Income - COGS
--   - Other Expenses (net, gst, gross)
--   - Net Profit = Income - COGS - Other Expenses
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_summary_monthly AS
WITH s AS (
    SELECT practitioner_id, period_month, section_type,
           SUM(net_amount) AS total_net, 
           SUM(gst_amount) AS total_gst, 
           SUM(gross_amount) AS total_gross
    FROM vw_pl_line_items 
    GROUP BY practitioner_id, period_month, section_type
)
SELECT practitioner_id, period_month,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='COLLECTION'),  0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type='COLLECTION'),  0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COLLECTION'),  0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='COST'),        0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type='COST'),        0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COST'),        0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='COST'),0) AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='OTHER_COST'),  0) AS other_expenses_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type='OTHER_COST'),  0) AS other_expenses_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='OTHER_COST'),  0) AS other_expenses_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='COST'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='OTHER_COST'),0) AS net_profit_net,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type='COST'),0) 
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type='OTHER_COST'),0) AS net_profit_gross
FROM s 
GROUP BY practitioner_id, period_month 
ORDER BY period_month;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_by_responsibility
-- P&L split by payment_responsibility (OWNER vs CLINIC)
--
-- Purpose: Track who is responsible for each expense/income
-- Usage: Filter by practitioner_id and period_month in queries
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_by_responsibility AS
SELECT practitioner_id, period_month, payment_responsibility, section_type, pl_section,
       account_code, account_name,
       SUM(net_amount) AS total_net, 
       SUM(gst_amount) AS total_gst, 
       SUM(gross_amount) AS total_gross,
       COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY practitioner_id, period_month, payment_responsibility, section_type, pl_section, account_code, account_name
ORDER BY period_month, payment_responsibility, pl_section, account_code;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_by_financial_year
-- P&L totals by financial year and quarter
--
-- Purpose: Align P&L reporting with financial year structure
-- Dependencies: tbl_financial_year, tbl_financial_quarter
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_by_financial_year AS
SELECT li.practitioner_id, 
       fy.id AS financial_year_id, fy.label AS financial_year,
       fq.id AS financial_quarter_id, fq.label AS quarter,
       li.pl_section, li.section_type, li.account_code, li.account_name, li.account_type,
       SUM(li.net_amount) AS total_net, 
       SUM(li.gst_amount) AS total_gst, 
       SUM(li.gross_amount) AS total_gross,
       COUNT(DISTINCT li.entry_id) AS entry_count
FROM vw_pl_line_items li
JOIN tbl_financial_year    fy ON li.submitted_at::DATE BETWEEN fy.start_date AND fy.end_date
JOIN tbl_financial_quarter fq ON li.submitted_at::DATE BETWEEN fq.start_date AND fq.end_date 
                              AND fq.financial_year_id = fy.id
GROUP BY li.practitioner_id, fy.id, fy.label, fq.id, fq.label,
         li.pl_section, li.section_type, li.account_code, li.account_name, li.account_type
ORDER BY financial_year, quarter, li.pl_section, li.account_code;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- VIEW: vw_pl_fy_summary
-- Financial year P&L summary (one row per FY per quarter)
--
-- Purpose: High-level P&L by financial year and quarter
-- Usage: Filter by practitioner_id, financial_year_id, or quarter
-- ============================================================
CREATE OR REPLACE VIEW vw_pl_fy_summary AS
WITH t AS (
    SELECT practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter, section_type,
           SUM(total_net) AS total_net, 
           SUM(total_gst) AS total_gst, 
           SUM(total_gross) AS total_gross
    FROM vw_pl_by_financial_year
    GROUP BY practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter, section_type
)
SELECT practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='COLLECTION'),  0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type='COLLECTION'),  0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COLLECTION'),  0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='COST'),        0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type='COST'),        0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COST'),        0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='COST'),0) AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type='OTHER_COST'),  0) AS other_expenses_net,
    COALESCE(SUM(total_net) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='COST'),0) 
        - COALESCE(SUM(total_net) FILTER (WHERE section_type='OTHER_COST'),0) AS net_profit_net,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type='COLLECTION'),0) 
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type='COST'),0) 
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type='OTHER_COST'),0) AS net_profit_gross
FROM t
GROUP BY practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter
ORDER BY financial_year, quarter;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- FUNCTION: fn_pl_date_range
-- Parameterized P&L for any date window, filtered by clinic_id
--
-- Purpose: Flexible P&L query for custom date ranges
-- Usage:
--   SELECT * FROM fn_pl_date_range(
--       '<clinic-uuid>', '2026-01-01', '2026-03-31'
--   );
-- ============================================================
CREATE OR REPLACE FUNCTION fn_pl_date_range(
    p_clinic_id UUID, 
    p_from_date DATE, 
    p_to_date DATE
)
RETURNS TABLE (
    pl_section TEXT, 
    account_code SMALLINT, 
    account_name VARCHAR, 
    account_type VARCHAR,
    payment_resp payment_responsibility, 
    tax_name VARCHAR, 
    tax_rate NUMERIC,
    total_net NUMERIC, 
    total_gst NUMERIC, 
    total_gross NUMERIC, 
    entry_count BIGINT
)
LANGUAGE SQL STABLE AS $fn$
    SELECT li.pl_section, li.account_code, li.account_name, li.account_type, li.payment_responsibility,
           li.tax_name, li.tax_rate,
           SUM(li.net_amount), SUM(li.gst_amount), SUM(li.gross_amount), COUNT(DISTINCT li.entry_id)
    FROM vw_pl_line_items li
    WHERE li.clinic_id = p_clinic_id 
      AND li.submitted_at::DATE BETWEEN p_from_date AND p_to_date
    GROUP BY li.pl_section, li.account_code, li.account_name, li.account_type,
             li.payment_responsibility, li.tax_name, li.tax_rate
    ORDER BY li.pl_section, li.account_code;
$fn$;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- FUNCTION: fn_pl_summary_date_range
-- Single-row P&L summary filtered by clinic_id + date range
--
-- Purpose: Quick P&L summary for custom date ranges
-- Usage:
--   SELECT * FROM fn_pl_summary_date_range(
--       '<clinic-uuid>', '2026-01-01', '2026-03-31'
--   );
-- ============================================================
CREATE OR REPLACE FUNCTION fn_pl_summary_date_range(
    p_clinic_id UUID, 
    p_from_date DATE, 
    p_to_date DATE
)
RETURNS TABLE (
    income_net NUMERIC, 
    income_gst NUMERIC, 
    income_gross NUMERIC,
    cogs_net NUMERIC, 
    cogs_gst NUMERIC, 
    cogs_gross NUMERIC,
    gross_profit_net NUMERIC, 
    other_expenses_net NUMERIC,
    net_profit_net NUMERIC, 
    net_profit_gross NUMERIC
)
LANGUAGE SQL STABLE AS $fn$
    SELECT
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type='COLLECTION'),  0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type='COLLECTION'),  0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type='COLLECTION'),  0),
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type='COST'),        0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type='COST'),        0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type='COST'),        0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type='COLLECTION'),0) 
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type='COST'),0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type='OTHER_COST'),0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type='COLLECTION'),0) 
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type='COST'),0) 
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type='OTHER_COST'),0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type='COLLECTION'),0) 
            - COALESCE(SUM(gross_amount) FILTER (WHERE section_type='COST'),0) 
            - COALESCE(SUM(gross_amount) FILTER (WHERE section_type='OTHER_COST'),0)
    FROM vw_pl_line_items
    WHERE clinic_id = p_clinic_id 
      AND submitted_at::DATE BETWEEN p_from_date AND p_to_date;
$fn$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS fn_pl_summary_date_range(UUID, DATE, DATE);
DROP FUNCTION IF EXISTS fn_pl_date_range(UUID, DATE, DATE);
DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;
DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;
DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;
DROP VIEW IF EXISTS vw_pl_by_account CASCADE;
DROP VIEW IF EXISTS vw_pl_line_items CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
-- +goose StatementEnd
