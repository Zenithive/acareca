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
--    1A   GST collected on taxable sales
--    G11  Total purchases (inc GST, excl BAS Excluded)
--    1B   GST credits on taxable purchases
--    Net  1A - 1B  (positive = payable to ATO, negative = refund)
-- ============================================================

CREATE VIEW vw_bas_summary AS
WITH base AS (
    SELECT
        l.clinic_id,
        l.practitioner_id,
        l.period_quarter,
        l.period_year,
        l.section_type,
        l.bas_category,
        l.net_amount,
        l.gst_amount,
        l.gross_amount,
        -- Convert clinic_share (int) to ratio (decimal)
        COALESCE(f.clinic_share::decimal / 100, 0.60) AS share_ratio
    FROM vw_bas_line_items l
    -- Join to tbl_form to get the specific shares for this entry
    LEFT JOIN tbl_form f ON l.form_id = f.id 
    WHERE l.bas_category != 'BAS_EXCLUDED'
),
summary_stats AS (
    SELECT
        clinic_id,
        practitioner_id,
        period_quarter,
        period_year,
        -- Row A: Gross Patient Fees
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type = 'COLLECTION'), 0) AS a_gross_revenue,
        -- Row B: Lab Fees Paid by Clinic (Total Gross)
        COALESCE(SUM(gross_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0) AS b_lab_fees,
        -- Row G: Actual GST on those Lab Fees
        COALESCE(SUM(gst_amount) FILTER (WHERE section_type IN ('COST', 'OTHER_COST')), 0) AS g_gst_on_labs,
        -- 1A: GST collected from patients (usually 0)
        COALESCE(SUM(gst_amount) FILTER (WHERE section_type = 'COLLECTION' AND bas_category = 'TAXABLE'), 0) AS label_1a,
        -- Get the share ratio for the group
        MAX(share_ratio) AS share_ratio
    FROM base
    GROUP BY clinic_id, practitioner_id, period_quarter, period_year
)
SELECT
    clinic_id,
    practitioner_id,
    period_quarter,
    period_year,

    -- ── G1: TOTAL SALES (ROW A) ──────────────────────────────────
    ROUND(a_gross_revenue::numeric, 2) AS g1_total_sales_gross,

    -- ── 1A: GST ON SALES ──────────────────────────────────────────
    ROUND(label_1a::numeric, 2) AS label_1a_gst_on_sales,

    -- ── 1B: GST ON PURCHASES (E + G) ──────────────────────────────
    -- E = (A - B) * Share * 10%
    -- G = GST on Labs
  ROUND(
        ((a_gross_revenue - b_lab_fees) * share_ratio * 0.10)::numeric, 
        2
    ) AS label_1b_gst_on_purchases,

    -- ── G11: TOTAL PURCHASES (F + B) ──────────────────────────────
    -- F = Service Fee Incl GST
    -- B = Lab Fees Gross
   ROUND(
        ((a_gross_revenue - b_lab_fees) * share_ratio * 1.10)::numeric, 
        2
    ) AS g11_total_purchases_gross,

    -- ── NET GST PAYABLE (1A - 1B) ─────────────────────────────────
    -- Negative result means a BAS Refund
    ROUND(
        (
            label_1a 
            - (((a_gross_revenue - b_lab_fees) * share_ratio * 0.10) + g_gst_on_labs)
        )::numeric, 2
    ) AS net_gst_payable,

    -- ── TOTALS FOR RECONCILIATION ─────────────────────────────────
    ROUND((a_gross_revenue - b_lab_fees)::numeric, 2) AS net_patient_fees,
    ROUND(((a_gross_revenue - b_lab_fees) * share_ratio)::numeric, 2) AS service_fee_net

FROM summary_stats;

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