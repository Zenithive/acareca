-- +goose Up
-- +goose StatementBegin

-- ============================================================
--  VIEW: vw_bas_line_items
--  Raw foundation — every SUBMITTED entry value with all
--  tax metadata needed for BAS categorisation.
--  BAS Excluded accounts (is_taxable=FALSE, rate=0) are
--  included here but flagged so the summary view can handle
--  them correctly.
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

    -- Field classification
    ff.id                                       AS form_field_id,
    ff.label                                    AS field_label,
    ff.section_type,                            -- COLLECTION | COST | OTHER_COST
    ff.payment_responsibility,
    ff.tax_type,                                -- INCLUSIVE | EXCLUSIVE | MANUAL

    -- COA
    coa.id                                      AS coa_id,
    coa.code                                    AS account_code,
    coa.name                                    AS account_name,

    -- Tax metadata (drives BAS categories)
    atx.id                                      AS account_tax_id,
    atx.name                                    AS tax_name,
    atx.rate                                    AS tax_rate,
    atx.is_taxable,

    -- BAS category flag
    -- 'TAXABLE'     → contributes to 1A / 1B and G1/G11
    -- 'GST_FREE'    → contributes to G1/G11 but NOT 1A/1B (rate=0, is_taxable=FALSE, not "BAS Excluded")
    -- 'BAS_EXCLUDED'→ excluded entirely from BAS reporting
    CASE
        WHEN atx.name = 'BAS Excluded'             THEN 'BAS_EXCLUDED'
        WHEN atx.is_taxable = TRUE                  THEN 'TAXABLE'
        ELSE                                             'GST_FREE'
    END                                         AS bas_category,

    -- Amounts (always stored as positive values in DB)
    COALESCE(fev.net_amount,   0)               AS net_amount,
    COALESCE(fev.gst_amount,   0)               AS gst_amount,
    COALESCE(fev.gross_amount, 0)               AS gross_amount

FROM tbl_form_entry_value    fev
JOIN tbl_form_entry          fe   ON fe.id   = fev.entry_id
JOIN tbl_form_field          ff   ON ff.id   = fev.form_field_id
JOIN tbl_custom_form_version cfv  ON cfv.id  = ff.form_version_id
JOIN tbl_form                f    ON f.id    = cfv.form_id
JOIN tbl_chart_of_accounts   coa  ON coa.id  = ff.coa_id
JOIN tbl_account_tax         atx  ON atx.id  = coa.account_tax_id

WHERE fe.status      = 'SUBMITTED'
  AND fe.deleted_at  IS NULL
  AND ff.deleted_at  IS NULL
  AND coa.deleted_at IS NULL;

-- +goose StatementEnd

-- +goose StatementBegin

-- ============================================================
--  VIEW: vw_bas_summary
--  Quarterly BAS summary per clinic — mirrors the ATO BAS form.
--
--  Key ATO labels:
--    G1   Total sales (inc GST, excl BAS Excluded)
--    G3   GST-free sales
--    G8   G1 - G3  (taxable sales)
--    1A   GST collected on taxable sales
--    G11  Total purchases (inc GST, excl BAS Excluded)
--    G14  GST-free purchases
--    1B   GST credits on taxable purchases
--    Net  1A - 1B  (positive = payable to ATO, negative = refund)
-- ============================================================
CREATE OR REPLACE VIEW vw_bas_summary AS
WITH base AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_month,
        period_quarter,
        period_year,
        section_type,
        bas_category,

        net_amount,
        gst_amount,
        gross_amount
    FROM vw_bas_line_items
    WHERE bas_category != 'BAS_EXCLUDED'  -- never reported on BAS
)
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,

    -- ── SALES ────────────────────────────────────────────────────
    -- G1: Total sales inc GST (taxable + GST-free, excl BAS Excluded)
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
        AS g1_total_sales_gross,

    -- G3: GST-free sales (no GST charged)
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0)
        AS g3_gst_free_sales,

    -- G8: Taxable sales (G1 − G3)
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
    - COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0)
        AS g8_taxable_sales,

    -- 1A: GST on sales (credits you collected, owed to ATO)
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)
        AS label_1a_gst_on_sales,

    -- ── PURCHASES ─────────────────────────────────────────────────
    -- G11: Total purchases inc GST (taxable + GST-free, excl BAS Excluded)
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)
        AS g11_total_purchases_gross,

    -- G14: GST-free purchases
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'GST_FREE'), 0)
        AS g14_gst_free_purchases,

    -- G15: Taxable purchases (G11 − G14)
    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)
    - COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'GST_FREE'), 0)
        AS g15_taxable_purchases,

    -- 1B: GST credits on purchases (reduces what you owe ATO)
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'TAXABLE'), 0)
        AS label_1b_gst_on_purchases,

    -- ── NET GST ───────────────────────────────────────────────────
    -- Positive = payable to ATO, Negative = ATO refund
    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION'               AND bas_category = 'TAXABLE'), 0)
  - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')    AND bas_category = 'TAXABLE'), 0)
        AS net_gst_payable,

    -- ── TOTALS ────────────────────────────────────────────────────
    -- Total net sales (ex-GST) — useful for income reconciliation
    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
        AS total_sales_net,

    -- Total net purchases (ex-GST)
    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)
        AS total_purchases_net

FROM base
GROUP BY clinic_id, practitioner_id, period_quarter, period_year
ORDER BY clinic_id, period_year, period_quarter;

-- +goose StatementEnd

-- +goose StatementBegin

-- ============================================================
--  VIEW: vw_bas_by_account
--  BAS breakdown per COA account per quarter.
--  Lets users see which accounts are driving their GST figures.
-- ============================================================
CREATE OR REPLACE VIEW vw_bas_by_account AS
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,
    section_type,
    bas_category,
    account_code,
    account_name,
    tax_name,
    tax_rate,

    -- Quantity of entries
    COUNT(DISTINCT entry_id)    AS entry_count,

    -- Amounts
    SUM(net_amount)             AS total_net,
    SUM(gst_amount)             AS total_gst,
    SUM(gross_amount)           AS total_gross

FROM vw_bas_line_items
WHERE bas_category != 'BAS_EXCLUDED'
GROUP BY
    clinic_id, practitioner_id,
    period_quarter, period_year,
    section_type, bas_category,
    account_code, account_name,
    tax_name, tax_rate
ORDER BY
    clinic_id, period_year, period_quarter,
    section_type, account_code;

-- +goose StatementEnd

-- +goose StatementBegin

-- ============================================================
--  VIEW: vw_bas_monthly
--  Same as vw_bas_summary but grouped by calendar month
--  instead of quarter — useful for dashboards and accrual tracking.
-- ============================================================
CREATE OR REPLACE VIEW vw_bas_monthly AS
WITH base AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_month,
        section_type,
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

    COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
        AS g1_total_sales_gross,

    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'GST_FREE'), 0)
        AS g3_gst_free_sales,

    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0)
        AS label_1a_gst_on_sales,

    COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)
        AS g11_total_purchases_gross,

    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'GST_FREE'), 0)
        AS g14_gst_free_purchases,

    COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'TAXABLE'), 0)
        AS label_1b_gst_on_purchases,

    COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION'            AND bas_category = 'TAXABLE'), 0)
  - COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST') AND bas_category = 'TAXABLE'), 0)
        AS net_gst_payable,

    COALESCE(SUM(net_amount) FILTER (WHERE section_type = 'COLLECTION'), 0)
        AS total_sales_net,

    COALESCE(SUM(net_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0)
        AS total_purchases_net

FROM base
GROUP BY clinic_id, practitioner_id, period_month
ORDER BY clinic_id, period_month;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
-- +goose StatementEnd