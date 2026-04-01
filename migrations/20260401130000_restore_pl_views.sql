-- +goose Up
-- +goose StatementBegin

-- ============================================================
--  Restore P&L views that were dropped by 20260401124500_bas_view.sql
--  These are rebuilt on top of vw_bas_line_items so both BAS
--  and P&L share the same underlying data source.
-- ============================================================

-- ------------------------------------------------------------
-- VIEW: vw_pl_line_items
-- Mirrors the original but sourced from vw_bas_line_items so
-- account_type filtering is preserved via the JOIN below.
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_line_items AS
SELECT
    bli.clinic_id,
    bli.practitioner_id,
    bli.form_id,
    bli.form_name,
    bli.entry_id,
    bli.submitted_at,
    bli.period_month,
    bli.period_quarter,
    bli.period_year,
    bli.form_field_id,
    bli.field_label,
    bli.section_type,
    bli.payment_responsibility,
    bli.tax_type,
    bli.coa_id,
    bli.account_code,
    bli.account_name,
    at.name                                     AS account_type,
    bli.tax_name,
    bli.tax_rate,
    bli.is_taxable,
    bli.net_amount,
    bli.gst_amount,
    bli.gross_amount,

    CASE
        WHEN bli.section_type = 'COLLECTION' THEN  bli.net_amount
        ELSE                                       -bli.net_amount
    END AS signed_net_amount,

    CASE
        WHEN bli.section_type = 'COLLECTION' THEN  bli.gross_amount
        ELSE                                       -bli.gross_amount
    END AS signed_gross_amount,

    CASE bli.section_type
        WHEN 'COLLECTION' THEN '1. Income'
        WHEN 'COST'       THEN '2. Cost of Sales'
        WHEN 'OTHER_COST' THEN '3. Other Expenses'
    END AS pl_section

FROM vw_bas_line_items bli
JOIN tbl_chart_of_accounts coa ON coa.id = bli.coa_id AND coa.deleted_at IS NULL
JOIN tbl_account_type      at  ON at.id  = coa.account_type_id
WHERE at.name IN ('Revenue', 'Expense');

-- ------------------------------------------------------------
-- VIEW: vw_pl_by_account
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_by_account AS
SELECT
    practitioner_id,
    period_month,
    pl_section,
    section_type,
    account_code,
    account_name,
    account_type,
    tax_name,
    tax_rate,
    SUM(net_amount)          AS total_net,
    SUM(gst_amount)          AS total_gst,
    SUM(gross_amount)        AS total_gross,
    SUM(signed_net_amount)   AS signed_net,
    SUM(signed_gross_amount) AS signed_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY
    practitioner_id, period_month,
    pl_section, section_type, account_code, account_name, account_type,
    tax_name, tax_rate
ORDER BY period_month, pl_section, account_code;

-- ------------------------------------------------------------
-- VIEW: vw_pl_summary_monthly
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_summary_monthly AS
WITH section_totals AS (
    SELECT
        practitioner_id,
        period_month,
        section_type,
        SUM(net_amount)   AS total_net,
        SUM(gst_amount)   AS total_gst,
        SUM(gross_amount) AS total_gross
    FROM vw_pl_line_items
    GROUP BY practitioner_id, period_month, section_type
)
SELECT
    practitioner_id,
    period_month,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),        0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'), 0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'), 0)    AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'OTHER_COST'), 0)  AS other_expenses_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'OTHER_COST'), 0)  AS other_expenses_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0)  AS other_expenses_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'),  0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'),  0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS net_profit_net,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),  0)
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),  0)
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS net_profit_gross
FROM section_totals
GROUP BY practitioner_id, period_month
ORDER BY period_month;

-- ------------------------------------------------------------
-- VIEW: vw_pl_by_responsibility
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_by_responsibility AS
SELECT
    practitioner_id,
    period_month,
    payment_responsibility,
    section_type,
    pl_section,
    account_code,
    account_name,
    SUM(net_amount)          AS total_net,
    SUM(gst_amount)          AS total_gst,
    SUM(gross_amount)        AS total_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY
    practitioner_id, period_month,
    payment_responsibility, section_type, pl_section,
    account_code, account_name
ORDER BY period_month, payment_responsibility, pl_section, account_code;

-- ------------------------------------------------------------
-- VIEW: vw_pl_by_financial_year
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_by_financial_year AS
SELECT
    li.practitioner_id,
    fy.id                       AS financial_year_id,
    fy.label                    AS financial_year,
    fq.id                       AS financial_quarter_id,
    fq.label                    AS quarter,
    li.pl_section,
    li.section_type,
    li.account_code,
    li.account_name,
    li.account_type,
    SUM(li.net_amount)          AS total_net,
    SUM(li.gst_amount)          AS total_gst,
    SUM(li.gross_amount)        AS total_gross,
    COUNT(DISTINCT li.entry_id) AS entry_count
FROM vw_pl_line_items li
JOIN tbl_financial_year    fy ON li.submitted_at::DATE BETWEEN fy.start_date AND fy.end_date
JOIN tbl_financial_quarter fq ON li.submitted_at::DATE BETWEEN fq.start_date AND fq.end_date
                              AND fq.financial_year_id = fy.id
GROUP BY
    li.practitioner_id,
    fy.id, fy.label,
    fq.id, fq.label,
    li.pl_section, li.section_type,
    li.account_code, li.account_name, li.account_type
ORDER BY financial_year, quarter, li.pl_section, li.account_code;

-- ------------------------------------------------------------
-- VIEW: vw_pl_fy_summary
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_fy_summary AS
WITH fy_totals AS (
    SELECT
        practitioner_id,
        financial_year_id,
        financial_year,
        financial_quarter_id,
        quarter,
        section_type,
        SUM(total_net)   AS total_net,
        SUM(total_gst)   AS total_gst,
        SUM(total_gross) AS total_gross
    FROM vw_pl_by_financial_year
    GROUP BY
        practitioner_id,
        financial_year_id, financial_year,
        financial_quarter_id, quarter,
        section_type
)
SELECT
    practitioner_id,
    financial_year_id,
    financial_year,
    financial_quarter_id,
    quarter,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COLLECTION'),   0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COLLECTION'),   0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),   0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COST'),         0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COST'),         0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),         0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'), 0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'), 0)     AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'OTHER_COST'),   0) AS other_expenses_net,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'),   0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'),   0)
        - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS net_profit_net,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),   0)
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),   0)
        - COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS net_profit_gross
FROM fy_totals
GROUP BY
    practitioner_id,
    financial_year_id, financial_year,
    financial_quarter_id, quarter
ORDER BY financial_year, quarter;

-- ------------------------------------------------------------
-- FUNCTION: fn_pl_date_range
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_pl_date_range(
    p_clinic_id   UUID,
    p_from_date   DATE,
    p_to_date     DATE
)
RETURNS TABLE (
    pl_section       TEXT,
    account_code     SMALLINT,
    account_name     VARCHAR,
    account_type     VARCHAR,
    payment_resp     payment_responsibility,
    tax_name         VARCHAR,
    tax_rate         NUMERIC,
    total_net        NUMERIC,
    total_gst        NUMERIC,
    total_gross      NUMERIC,
    entry_count      BIGINT
)
LANGUAGE SQL STABLE AS $fn$
    SELECT
        li.pl_section,
        li.account_code,
        li.account_name,
        li.account_type,
        li.payment_responsibility,
        li.tax_name,
        li.tax_rate,
        SUM(li.net_amount)          AS total_net,
        SUM(li.gst_amount)          AS total_gst,
        SUM(li.gross_amount)        AS total_gross,
        COUNT(DISTINCT li.entry_id) AS entry_count
    FROM vw_pl_line_items li
    WHERE li.clinic_id = p_clinic_id
      AND li.submitted_at::DATE BETWEEN p_from_date AND p_to_date
    GROUP BY
        li.pl_section, li.account_code, li.account_name,
        li.account_type, li.payment_responsibility,
        li.tax_name, li.tax_rate
    ORDER BY li.pl_section, li.account_code;
$fn$;

-- ------------------------------------------------------------
-- FUNCTION: fn_pl_summary_date_range
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION fn_pl_summary_date_range(
    p_clinic_id   UUID,
    p_from_date   DATE,
    p_to_date     DATE
)
RETURNS TABLE (
    income_net         NUMERIC,
    income_gst         NUMERIC,
    income_gross       NUMERIC,
    cogs_net           NUMERIC,
    cogs_gst           NUMERIC,
    cogs_gross         NUMERIC,
    gross_profit_net   NUMERIC,
    other_expenses_net NUMERIC,
    net_profit_net     NUMERIC,
    net_profit_gross   NUMERIC
)
LANGUAGE SQL STABLE AS $fn$
    SELECT
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COST'), 0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'),  0)
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COST'),  0)
            - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'),  0)
            - COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COST'),  0)
            - COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0)
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
-- +goose StatementEnd
