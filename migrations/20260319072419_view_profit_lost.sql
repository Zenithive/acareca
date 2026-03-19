-- +goose Up
-- +goose StatementBegin
-- ============================================================
--  Profit & Loss Views — acareca schema
--  Based on: tbl_chart_of_accounts, tbl_form_field,
--            tbl_form_entry, tbl_form_entry_value,
--            tbl_account_type, tbl_form
--
--  Design: views are clinic-agnostic (no clinic_id grouping).
--          clinic_id is passed as a filter via the functions
--          fn_pl_date_range / fn_pl_summary_date_range.
-- ============================================================

-- ------------------------------------------------------------
-- VIEW 1: vw_pl_line_items
-- Every submitted entry value joined to its COA account.
-- clinic_id is retained here only so functions can filter on it.
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_line_items AS
SELECT
    -- clinic_id kept for function-level filtering only
    fe.clinic_id,
    cfv.practitioner_id,

    -- Form / entry metadata
    f.id                            AS form_id,
    f.name                          AS form_name,
    f.method                        AS calculation_method,
    fe.id                           AS entry_id,
    fe.submitted_at,
    DATE_TRUNC('month',   fe.submitted_at) AS period_month,
    DATE_TRUNC('quarter', fe.submitted_at) AS period_quarter,
    DATE_TRUNC('year',    fe.submitted_at) AS period_year,

    -- Field / COA
    ff.id                           AS form_field_id,
    ff.label                        AS field_label,
    ff.section_type,
    ff.payment_responsibility,
    ff.tax_type,

    -- Chart of Accounts
    coa.id                          AS coa_id,
    coa.code                        AS account_code,
    coa.name                        AS account_name,
    at.name                         AS account_type,

    -- Tax details
    atx.name                        AS tax_name,
    atx.rate                        AS tax_rate,
    atx.is_taxable,

    -- Amounts
    COALESCE(fev.net_amount,   0)   AS net_amount,
    COALESCE(fev.gst_amount,   0)   AS gst_amount,
    COALESCE(fev.gross_amount, 0)   AS gross_amount,

    CASE
        WHEN ff.section_type = 'COLLECTION' THEN  COALESCE(fev.net_amount,   0)
        ELSE                                      -COALESCE(fev.net_amount,   0)
    END AS signed_net_amount,

    CASE
        WHEN ff.section_type = 'COLLECTION' THEN  COALESCE(fev.gross_amount, 0)
        ELSE                                      -COALESCE(fev.gross_amount, 0)
    END AS signed_gross_amount,

    CASE ff.section_type
        WHEN 'COLLECTION' THEN '1. Income'
        WHEN 'COST'       THEN '2. Cost of Sales'
        WHEN 'OTHER_COST' THEN '3. Other Expenses'
    END AS pl_section

FROM tbl_form_entry_value    fev
JOIN tbl_form_entry          fe  ON fe.id          = fev.entry_id
JOIN tbl_form_field          ff  ON ff.id          = fev.form_field_id
JOIN tbl_custom_form_version cfv ON cfv.id         = ff.form_version_id
JOIN tbl_form                f   ON f.id           = cfv.form_id
JOIN tbl_chart_of_accounts   coa ON coa.id         = ff.coa_id
JOIN tbl_account_type        at  ON at.id          = coa.account_type_id
JOIN tbl_account_tax         atx ON atx.id         = coa.account_tax_id

WHERE fe.status    = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND coa.deleted_at IS NULL
  AND at.name IN ('Revenue', 'Expense');


-- ------------------------------------------------------------
-- VIEW 2: vw_pl_by_account
-- Totals per COA account grouped by month.
-- Filter by clinic_id in your query: WHERE clinic_id = '...'
-- ------------------------------------------------------------
CREATE OR REPLACE VIEW vw_pl_by_account AS
SELECT
    practitioner_id,
    period_month,
    pl_section,
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
    pl_section, account_code, account_name, account_type,
    tax_name, tax_rate
ORDER BY period_month, pl_section, account_code;


-- ------------------------------------------------------------
-- VIEW 3: vw_pl_summary_monthly
-- P&L summary by month (Income → COGS → Gross Profit → Net Profit).
-- Filter by clinic_id in your query: WHERE clinic_id = '...'
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
-- VIEW 4: vw_pl_by_responsibility
-- Splits P&L by payment_responsibility (OWNER vs CLINIC).
-- Filter by clinic_id in your query: WHERE clinic_id = '...'
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
-- VIEW 5: vw_pl_by_financial_year
-- Totals joined to tbl_financial_year + tbl_financial_quarter.
-- Filter by clinic_id in your query: WHERE clinic_id = '...'
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
-- VIEW 6: vw_pl_fy_summary
-- One-row-per-FY-per-quarter P&L summary.
-- Filter by clinic_id in your query: WHERE clinic_id = '...'
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
-- Parameterised P&L for any date window, filtered by clinic_id.
-- Usage:
--   SELECT * FROM fn_pl_date_range(
--       '<clinic-uuid>', '2026-01-01', '2026-03-31'
--   );
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
LANGUAGE SQL STABLE AS $$
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
$$;


-- ------------------------------------------------------------
-- FUNCTION: fn_pl_summary_date_range
-- Single-row P&L summary filtered by clinic_id + date range.
-- Usage:
--   SELECT * FROM fn_pl_summary_date_range(
--       '<clinic-uuid>', '2026-01-01', '2026-03-31'
--   );
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
LANGUAGE SQL STABLE AS $$
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
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS fn_pl_summary_date_range(UUID, DATE, DATE);
DROP FUNCTION IF EXISTS fn_pl_date_range(UUID, DATE, DATE);
DROP VIEW IF EXISTS vw_pl_fy_summary;
DROP VIEW IF EXISTS vw_pl_by_financial_year;
DROP VIEW IF EXISTS vw_pl_by_responsibility;
DROP VIEW IF EXISTS vw_pl_summary_monthly;
DROP VIEW IF EXISTS vw_pl_by_account;
DROP VIEW IF EXISTS vw_pl_line_items;
-- +goose StatementEnd
