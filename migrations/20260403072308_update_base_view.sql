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
  AND coa.deleted_at IS NULL
  AND coa.is_system  = FALSE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
-- +goose StatementEnd