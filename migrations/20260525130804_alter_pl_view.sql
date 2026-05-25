-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;

DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;

DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;

DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;

DROP VIEW IF EXISTS vw_pl_by_account CASCADE;

DROP VIEW IF EXISTS vw_pl_line_items CASCADE;

CREATE OR REPLACE VIEW vw_pl_line_items AS
SELECT
    fe.clinic_id,
    cfv.practitioner_id,
    f.id AS form_id,
    f.name AS form_name,
    f.method AS calculation_method,
    fe.id AS entry_id,
    CASE
        WHEN f.method = 'EXPENSE_ENTRY' THEN fev.date::date
        ELSE fe."date"::date
    END AS date,
    DATE_TRUNC(
        'month',
        COALESCE(
            CASE
                WHEN f.method = 'EXPENSE_ENTRY' THEN fev.date::date
                ELSE fe."date"::date
            END,
            fe.created_at
        )
    ) AS period_month,
    DATE_TRUNC(
        'quarter',
        COALESCE(
            CASE
                WHEN f.method = 'EXPENSE_ENTRY' THEN fev.date::date
                ELSE fe."date"::date
            END,
            fe.created_at
        )
    ) AS period_quarter,
    DATE_TRUNC(
        'year',
        COALESCE(
            CASE
                WHEN f.method = 'EXPENSE_ENTRY' THEN fev.date::date
                ELSE fe."date"::date
            END,
            fe.created_at
        )
    ) AS period_year,
    ff.id AS form_field_id,
    ff.label AS field_label,
    ff.section_type,
    ff.payment_responsibility,
    ff.tax_type,
    ff.business_use,
    coa.id AS coa_id,
    coa.code AS account_code,
    coa.name AS account_name,
    at.name AS account_type,
    atx.name AS tax_name,
    atx.rate AS tax_rate,
    atx.is_taxable,
    COALESCE(fev.net_amount, 0) AS net_amount,
    COALESCE(fev.gst_amount, 0) AS gst_amount,
    COALESCE(fev.gross_amount, 0) AS gross_amount,
    CASE
        WHEN at.name = 'Revenue' THEN COALESCE(fev.net_amount, 0)
        ELSE - COALESCE(fev.net_amount, 0)
    END AS signed_net_amount,
    CASE
        WHEN at.name = 'Revenue' THEN COALESCE(fev.gross_amount, 0)
        ELSE - COALESCE(fev.gross_amount, 0)
    END AS signed_gross_amount,
    CASE
        WHEN at.name = 'Revenue' THEN '1. Income'
        WHEN at.name = 'Expense'
        AND ff.section_type = 'OTHER_COST' THEN '3. Other Costs'
        WHEN at.name = 'Expense' THEN '2. Cost of Sales'
        ELSE '2. Cost of Sales' -- Default fallback for any other expense types
    END AS pl_section
FROM
    tbl_form_entry fe
    JOIN tbl_custom_form_version cfv ON cfv.id = fe.form_version_id
    JOIN tbl_form f ON f.id = cfv.form_id
    JOIN (
        SELECT DISTINCT
            ON (entry_id, form_field_id) id,
            entry_id,
            form_field_id,
            net_amount,
            gst_amount,
            gross_amount,
            description,
            date
        FROM tbl_form_entry_value
        ORDER BY entry_id, form_field_id, COALESCE(updated_at, created_at) DESC
    ) fev ON fev.entry_id = fe.id
    JOIN tbl_form_field ff ON ff.id = fev.form_field_id
    JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id
    JOIN tbl_account_type at ON at.id = coa.account_type_id
    LEFT JOIN tbl_account_tax atx ON atx.id = coa.account_tax_id
WHERE
    fe.status = 'SUBMITTED'
    AND fe.deleted_at IS NULL
    AND ff.deleted_at IS NULL
    AND ff.is_formula = FALSE
    AND coa.deleted_at IS NULL
    AND at.name IN ('Revenue', 'Expense')
    AND ff.coa_id IS NOT NULL;

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
    SUM(net_amount) AS total_net,
    SUM(gst_amount) AS total_gst,
    SUM(gross_amount) AS total_gross,
    SUM(signed_net_amount) AS signed_net,
    SUM(signed_gross_amount) AS signed_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY
    practitioner_id,
    period_month,
    pl_section,
    section_type,
    account_code,
    account_name,
    account_type,
    tax_name,
    tax_rate
ORDER BY
    period_month,
    pl_section,
    account_code;

CREATE OR REPLACE VIEW vw_pl_summary_monthly AS
WITH
    s AS (
        SELECT
            practitioner_id,
            period_month,
            account_type,
            pl_section,
            SUM(net_amount) AS total_net,
            SUM(gst_amount) AS total_gst,
            SUM(gross_amount) AS total_gross
        FROM vw_pl_line_items
        GROUP BY
            practitioner_id,
            period_month,
            account_type,
            pl_section
    )
SELECT
    practitioner_id,
    period_month,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_net,
    COALESCE(
        SUM(total_gst) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_gst,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_gross,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_net,
    COALESCE(
        SUM(total_gst) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_gst,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_gross,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS gross_profit_net,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '3. Other Costs'
        ),
        0
    ) AS other_costs_net,
    COALESCE(
        SUM(total_gst) FILTER (
            WHERE
                pl_section = '3. Other Costs'
        ),
        0
    ) AS other_costs_gst,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                pl_section = '3. Other Costs'
        ),
        0
    ) AS other_costs_gross,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Expense'
        ),
        0
    ) AS net_profit_net,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Expense'
        ),
        0
    ) AS net_profit_gross
FROM s
GROUP BY
    practitioner_id,
    period_month
ORDER BY period_month;

CREATE OR REPLACE VIEW vw_pl_by_responsibility AS
SELECT
    practitioner_id,
    period_month,
    payment_responsibility,
    section_type,
    pl_section,
    account_code,
    account_name,
    SUM(net_amount) AS total_net,
    SUM(gst_amount) AS total_gst,
    SUM(gross_amount) AS total_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY
    practitioner_id,
    period_month,
    payment_responsibility,
    section_type,
    pl_section,
    account_code,
    account_name
ORDER BY
    period_month,
    payment_responsibility,
    pl_section,
    account_code;

CREATE OR REPLACE VIEW vw_pl_by_financial_year AS
SELECT
    li.practitioner_id,
    fy.id AS financial_year_id,
    fy.label AS financial_year,
    fq.id AS financial_quarter_id,
    fq.label AS quarter,
    li.pl_section,
    li.section_type,
    li.account_code,
    li.account_name,
    li.account_type,
    SUM(li.net_amount) AS total_net,
    SUM(li.gst_amount) AS total_gst,
    SUM(li.gross_amount) AS total_gross,
    COUNT(DISTINCT li.entry_id) AS entry_count
FROM vw_pl_line_items li
JOIN tbl_financial_year    fy ON li.date BETWEEN fy.start_date AND fy.end_date
JOIN tbl_financial_quarter fq ON li.date BETWEEN fq.start_date AND fq.end_date
                              AND fq.financial_year_id = fy.id
GROUP BY
    li.practitioner_id,
    fy.id, fy.label,
    fq.id, fq.label,
    li.pl_section, li.section_type,
    li.account_code, li.account_name, li.account_type
ORDER BY financial_year, quarter, li.pl_section, li.account_code;
CREATE OR REPLACE VIEW vw_pl_fy_summary AS
WITH
    t AS (
        SELECT
            practitioner_id,
            financial_year_id,
            financial_year,
            financial_quarter_id,
            quarter,
            account_type,
            pl_section,
            SUM(total_net) AS total_net,
            SUM(total_gst) AS total_gst,
            SUM(total_gross) AS total_gross
        FROM vw_pl_by_financial_year
        GROUP BY
            practitioner_id,
            financial_year_id,
            financial_year,
            financial_quarter_id,
            quarter,
            account_type,
            pl_section
    )
SELECT
    practitioner_id,
    financial_year_id,
    financial_year,
    financial_quarter_id,
    quarter,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_net,
    COALESCE(
        SUM(total_gst) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_gst,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) AS income_gross,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_net,
    COALESCE(
        SUM(total_gst) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_gst,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS cogs_gross,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '2. Cost of Sales'
        ),
        0
    ) AS gross_profit_net,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                pl_section = '3. Other Costs'
        ),
        0
    ) AS other_costs_net,
    COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_net) FILTER (
            WHERE
                account_type = 'Expense'
        ),
        0
    ) AS net_profit_net,
    COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Revenue'
        ),
        0
    ) - COALESCE(
        SUM(total_gross) FILTER (
            WHERE
                account_type = 'Expense'
        ),
        0
    ) AS net_profit_gross
FROM t
GROUP BY
    practitioner_id,
    financial_year_id,
    financial_year,
    financial_quarter_id,
    quarter
ORDER BY financial_year, quarter;

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
      AND li.date BETWEEN p_from_date AND p_to_date
    GROUP BY li.pl_section, li.account_code, li.account_name, li.account_type,
             li.payment_responsibility, li.tax_name, li.tax_rate
    ORDER BY li.pl_section, li.account_code;
$fn$;

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
    other_costs_net NUMERIC,
    net_profit_net NUMERIC,
    net_profit_gross NUMERIC
)
LANGUAGE SQL STABLE AS $fn$
    SELECT
        COALESCE(SUM(net_amount)   FILTER (WHERE account_type = 'Revenue'), 0) AS income_net,
        COALESCE(SUM(gst_amount)   FILTER (WHERE account_type = 'Revenue'), 0) AS income_gst,
        COALESCE(SUM(gross_amount) FILTER (WHERE account_type = 'Revenue'), 0) AS income_gross,
        COALESCE(SUM(net_amount)   FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS cogs_net,
        COALESCE(SUM(gst_amount)   FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS cogs_gst,
        COALESCE(SUM(gross_amount) FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS cogs_gross,
        COALESCE(SUM(net_amount)   FILTER (WHERE account_type = 'Revenue'), 0)
            - COALESCE(SUM(net_amount) FILTER (WHERE pl_section = '2. Cost of Sales'), 0) AS gross_profit_net,
        COALESCE(SUM(net_amount)   FILTER (WHERE pl_section = '3. Other Costs'), 0) AS other_costs_net,
        COALESCE(SUM(net_amount)   FILTER (WHERE account_type = 'Revenue'), 0)
            - COALESCE(SUM(net_amount) FILTER (WHERE account_type = 'Expense'), 0) AS net_profit_net,
        COALESCE(SUM(gross_amount) FILTER (WHERE account_type = 'Revenue'), 0)
            - COALESCE(SUM(gross_amount) FILTER (WHERE account_type = 'Expense'), 0) AS net_profit_gross
    FROM vw_pl_line_items
    WHERE clinic_id = p_clinic_id
      AND "date" BETWEEN p_from_date AND p_to_date;
$fn$;


-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP FUNCTION IF EXISTS fn_pl_summary_date_range CASCADE;
DROP FUNCTION IF EXISTS fn_pl_date_range CASCADE;

DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;
DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;
DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;
DROP VIEW IF EXISTS vw_pl_by_account CASCADE;
DROP VIEW IF EXISTS vw_pl_line_items CASCADE;

-- +goose StatementEnd