-- +goose Up
-- +goose StatementBegin

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
CREATE VIEW vw_pl_line_items AS
SELECT
    fe.clinic_id,
    cfv.practitioner_id,
    f.id                                 AS form_id,
    f.name                               AS form_name,
    f.method                             AS calculation_method,
    fe.id                                AS entry_id,
    fe.submitted_at,
    fe.date                              AS transaction_date,
    DATE_TRUNC('month',   COALESCE(fe.date::timestamp, fe.submitted_at, fe.created_at)) AS period_month,
    DATE_TRUNC('quarter', COALESCE(fe.date::timestamp, fe.submitted_at, fe.created_at)) AS period_quarter,
    DATE_TRUNC('year',    COALESCE(fe.date::timestamp, fe.submitted_at, fe.created_at)) AS period_year,
    ff.id                                AS form_field_id,
    ff.label                             AS field_label,
    ff.section_type,
    ff.payment_responsibility,
    ff.tax_type   AS tax_type,
    ff.business_use AS business_use,
    coa.id                               AS coa_id,
    coa.code                             AS account_code,
    coa.name                             AS account_name,
    at.name                              AS account_type,
    atx.name                             AS tax_name,
    atx.rate                             AS tax_rate,
    atx.is_taxable,
    COALESCE(fev.net_amount,   0)        AS net_amount,
    COALESCE(fev.gst_amount,   0)        AS gst_amount,
    COALESCE(fev.gross_amount, 0)        AS gross_amount,
    CASE
        WHEN ff.section_type = 'COLLECTION' THEN  COALESCE(fev.net_amount,   0)
        ELSE                                     -COALESCE(fev.net_amount,   0)
    END AS signed_net_amount,
    CASE
        WHEN ff.section_type = 'COLLECTION' THEN  COALESCE(fev.gross_amount, 0)
        ELSE                                     -COALESCE(fev.gross_amount, 0)
    END AS signed_gross_amount,
    CASE ff.section_type
        WHEN 'COLLECTION' THEN '1. Income'
        WHEN 'COST'       THEN '2. Cost of Sales'
        WHEN 'OTHER_COST' THEN '3. Other Expenses'
        WHEN 'EXPENSE_ENTRY' THEN '3. Expense Entries'
    END AS pl_section
FROM tbl_form_entry fe
JOIN tbl_custom_form_version cfv ON cfv.id = fe.form_version_id
JOIN tbl_form f ON f.id = cfv.form_id
JOIN (
    SELECT DISTINCT ON (entry_id, form_field_id)
        id, entry_id, form_field_id, net_amount, gst_amount, gross_amount, description
    FROM tbl_form_entry_value
    WHERE updated_at IS NULL  
    ORDER BY entry_id, form_field_id, created_at DESC
) fev ON fev.entry_id = fe.id
JOIN tbl_form_field ff ON ff.id = fev.form_field_id
JOIN tbl_chart_of_accounts coa ON coa.id = ff.coa_id
JOIN tbl_account_type at ON at.id = coa.account_type_id
LEFT JOIN tbl_account_tax atx ON atx.id = coa.account_tax_id
WHERE fe.status = 'SUBMITTED'
  AND fe.deleted_at IS NULL
  AND ff.deleted_at IS NULL
  AND ff.is_formula = FALSE
  AND coa.deleted_at IS NULL
  AND at.name IN ('Revenue', 'Expense')
  AND ff.coa_id IS NOT NULL;

-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE VIEW vw_pl_by_account AS
SELECT
    clinic_id, practitioner_id, period_month, pl_section,
    account_code, account_name, account_type, tax_name, tax_rate,
    SUM(net_amount)          AS total_net,
    SUM(gst_amount)          AS total_gst,
    SUM(gross_amount)        AS total_gross,
    SUM(signed_net_amount)   AS signed_net,
    SUM(signed_gross_amount) AS signed_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY clinic_id, practitioner_id, period_month, pl_section, account_code, account_name, account_type, tax_name, tax_rate
ORDER BY period_month, pl_section, account_code;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE VIEW vw_pl_summary_monthly AS
WITH section_totals AS (
    SELECT clinic_id, practitioner_id, period_month, section_type,
           SUM(net_amount) AS total_net, SUM(gst_amount) AS total_gst, SUM(gross_amount) AS total_gross,
           SUM(signed_net_amount) AS sg_net_amount, SUM(signed_gross_amount) AS sg_gross_amount
    FROM vw_pl_line_items
    GROUP BY clinic_id, practitioner_id, period_month, section_type
)
SELECT
    clinic_id, practitioner_id, period_month,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),        0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'), 0) AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gross,
    COALESCE(SUM(sg_net_amount), 0) AS net_profit_net,
    COALESCE(SUM(sg_gross_amount), 0) AS net_profit_gross
FROM section_totals
GROUP BY clinic_id, practitioner_id, period_month
ORDER BY period_month;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE VIEW vw_pl_by_responsibility AS
SELECT
    clinic_id, practitioner_id, period_month, payment_responsibility, section_type, pl_section, account_code, account_name,
    SUM(net_amount) AS total_net, SUM(gst_amount) AS total_gst, SUM(gross_amount) AS total_gross,
    SUM(signed_net_amount) AS signed_net, SUM(signed_gross_amount) AS signed_gross,
    COUNT(DISTINCT entry_id) AS entry_count
FROM vw_pl_line_items
GROUP BY clinic_id, practitioner_id, period_month, payment_responsibility, section_type, pl_section, account_code, account_name
ORDER BY period_month, pl_section, account_code;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE VIEW vw_pl_by_financial_year AS
SELECT
    li.clinic_id,
    li.practitioner_id,
    fy.id AS financial_year_id, fy.label AS financial_year,
    fq.id AS financial_quarter_id, fq.label AS quarter,
    li.pl_section, li.section_type, li.account_code, li.account_name, li.account_type,
    SUM(li.net_amount) AS total_net, SUM(li.gst_amount) AS total_gst, SUM(li.gross_amount) AS total_gross,
    SUM(li.signed_net_amount) AS signed_net, SUM(li.signed_gross_amount) AS signed_gross
FROM vw_pl_line_items li
JOIN tbl_financial_year    fy ON li.period_month::DATE BETWEEN fy.start_date AND fy.end_date
JOIN tbl_financial_quarter fq ON li.period_month::DATE BETWEEN fq.start_date AND fq.end_date AND fq.financial_year_id = fy.id
GROUP BY li.clinic_id, li.practitioner_id, fy.id, fy.label, fq.id, fq.label, li.pl_section, li.section_type, li.account_code, li.account_name, li.account_type
ORDER BY fy.start_date, fq.label, li.pl_section, li.account_code;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE VIEW vw_pl_fy_summary AS
WITH fy_totals AS (
    SELECT clinic_id, practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter, section_type,
           SUM(total_net) AS total_net, SUM(total_gst) AS total_gst, SUM(total_gross) AS total_gross,
           SUM(signed_net) AS sg_net_amount, SUM(signed_gross) AS sg_gross_amount
    FROM vw_pl_by_financial_year
    GROUP BY clinic_id, practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter, section_type
)
SELECT
    clinic_id, practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COLLECTION'),  0) AS income_gross,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'COST'),        0) AS cogs_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'COST'),        0) AS cogs_gross,
    COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(total_net) FILTER (WHERE section_type = 'COST'), 0) AS gross_profit_net,
    COALESCE(SUM(total_net)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_net,
    COALESCE(SUM(total_gst)   FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gst,
    COALESCE(SUM(total_gross) FILTER (WHERE section_type = 'OTHER_COST'), 0) AS other_expenses_gross,
    COALESCE(SUM(sg_net_amount), 0) AS net_profit_net,
    COALESCE(SUM(sg_gross_amount), 0) AS net_profit_gross
FROM fy_totals
GROUP BY clinic_id, practitioner_id, financial_year_id, financial_year, financial_quarter_id, quarter
ORDER BY financial_year_id, quarter;
-- +goose StatementEnd


-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fn_pl_date_range(p_clinic_id UUID, p_from_date DATE, p_to_date DATE)
RETURNS TABLE (pl_section TEXT, account_code SMALLINT, account_name VARCHAR, account_type VARCHAR, payment_resp payment_responsibility, tax_name VARCHAR, tax_rate NUMERIC, total_net NUMERIC, total_gst NUMERIC, total_gross NUMERIC, entry_count BIGINT)
LANGUAGE SQL STABLE AS $$
    SELECT li.pl_section, li.account_code, li.account_name, li.account_type, li.payment_responsibility,
           li.tax_name, li.tax_rate,
           SUM(li.net_amount), SUM(li.gst_amount), SUM(li.gross_amount), COUNT(DISTINCT li.entry_id)
    FROM vw_pl_line_items li
    WHERE li.clinic_id = p_clinic_id AND li.submitted_at::DATE BETWEEN p_from_date AND p_to_date
    GROUP BY li.pl_section, li.account_code, li.account_name, li.account_type, li.payment_responsibility, li.tax_name, li.tax_rate
    ORDER BY li.pl_section, li.account_code;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fn_pl_summary_date_range(p_clinic_id UUID, p_from_date DATE, p_to_date DATE)
RETURNS TABLE (income_net NUMERIC, income_gst NUMERIC, income_gross NUMERIC, cogs_net NUMERIC, cogs_gst NUMERIC, cogs_gross NUMERIC, gross_profit_net NUMERIC, other_expenses_net NUMERIC, net_profit_net NUMERIC, net_profit_gross NUMERIC)
LANGUAGE SQL STABLE AS $$
    SELECT
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'),  0),
        COALESCE(SUM(net_amount)   FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(gst_amount)   FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COST'),        0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COST'), 0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0),
        COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COST'), 0) - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0),
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) - COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COST'), 0) - COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'OTHER_COST'), 0)
    FROM vw_pl_line_items
    WHERE clinic_id = p_clinic_id AND submitted_at::DATE BETWEEN p_from_date AND p_to_date;
$$;
-- +goose StatementEnd



-- +goose StatementBegin

CREATE OR REPLACE VIEW vw_bas_line_items AS
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
-- +goose StatementEnd
  

-- +goose StatementBegin
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
-- +goose StatementEnd



-- +goose StatementBegin
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
-- +goose StatementEnd


-- +goose StatementBegin
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