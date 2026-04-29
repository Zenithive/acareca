-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;


CREATE OR REPLACE VIEW vw_balance_sheet_line_items AS
SELECT fe.clinic_id,
    COALESCE(cfv.practitioner_id, p.id) AS practitioner_id,
    fe.id AS entry_id,
    fe.submitted_at,
    fe.date AS entry_date,
    DATE_TRUNC('month', fe.submitted_at) AS period_month,
    DATE_TRUNC('year', fe.submitted_at) AS period_year,
    ff.id AS form_field_id,
    ff.section_type,
    COALESCE(fev.coa_id, ff.coa_id) AS coa_id,
    coa.code AS account_code,
    coa.name AS account_name,
    at.id AS account_type_id,
    at.name AS account_type,
    COALESCE(fev.net_amount, 0) AS net_amount,
    COALESCE(fev.gross_amount, 0) AS gross_amount,
    fev.description,
    CASE
        WHEN coa.code = 880 THEN - COALESCE(fev.net_amount, 0) -- Drawings (reduces equity)
        WHEN coa.code = 881 THEN COALESCE(fev.net_amount, 0) -- Funds Introduced (increases equity)
        WHEN coa.code = 970 THEN COALESCE(fev.net_amount, 0) -- Share Capital (increases equity)
        WHEN at.name = 'Asset' THEN COALESCE(fev.net_amount, 0)
        WHEN at.name = 'Liability' THEN COALESCE(fev.net_amount, 0)
        ELSE COALESCE(fev.net_amount, 0)
    END AS signed_amount
FROM tbl_form_entry fe
    LEFT JOIN tbl_custom_form_version cfv ON cfv.id = fe.form_version_id
    LEFT JOIN tbl_clinic c ON c.id = fe.clinic_id
    LEFT JOIN tbl_practitioner p ON p.id = c.practitioner_id
    JOIN (
        SELECT DISTINCT ON (
                entry_id,
                COALESCE(form_field_id::text, coa_id::text)
            ) id,
            entry_id,
            form_field_id,
            coa_id,
            net_amount,
            gst_amount,
            gross_amount,
            description
        FROM tbl_form_entry_value
        WHERE updated_at IS NULL -- Only non-deleted values
        ORDER BY entry_id,
            COALESCE(form_field_id::text, coa_id::text),
            created_at DESC
    ) fev ON fev.entry_id = fe.id
    LEFT JOIN tbl_form_field ff ON ff.id = fev.form_field_id -- LEFT JOIN for direct COA entries
    JOIN tbl_chart_of_accounts coa ON coa.id = COALESCE(fev.coa_id, ff.coa_id) -- Use direct coa_id if form_field is NULL
    JOIN tbl_account_type at ON at.id = coa.account_type_id
WHERE fe.status = 'SUBMITTED'
    AND fe.deleted_at IS NULL
    AND coa.deleted_at IS NULL
    AND at.name IN ('Asset', 'Liability', 'Equity') -- Only balance sheet accounts
    AND (
        (
            ff.id IS NOT NULL
            AND ff.deleted_at IS NULL
            AND ff.is_formula = FALSE
        )
        OR (
            ff.id IS NULL
            AND fev.coa_id IS NOT NULL
        ) -- Direct COA entry
    );



CREATE OR REPLACE VIEW vw_balance_sheet_summary AS
SELECT practitioner_id,
    clinic_id,
    account_type,
    account_code,
    account_name,
    coa_id,
    SUM(signed_amount) AS balance,
    COUNT(DISTINCT entry_id) AS entry_count,
    MAX(submitted_at) AS last_transaction_date
FROM vw_balance_sheet_line_items
GROUP BY practitioner_id,
    clinic_id,
    account_type,
    account_code,
    account_name,
    coa_id
ORDER BY account_type,
    account_code;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
-- +goose StatementEnd