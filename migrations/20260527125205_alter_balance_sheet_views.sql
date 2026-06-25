-- +goose Up
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
DROP VIEW IF EXISTS vw_double_entry_entry_summary CASCADE;
DROP VIEW IF EXISTS vw_double_entry_line_items CASCADE;

CREATE OR REPLACE VIEW vw_double_entry_line_items AS
SELECT fe.clinic_id,
    COALESCE(cfv.practitioner_id, p.id) AS practitioner_id,
    fe.submitted_by AS user_id,
    fe.submitted_by,
    fe.id AS entry_id,
    fe."date"::date,
    fe.date AS entry_date,
    DATE_TRUNC('month', fe.date) AS period_month,
    DATE_TRUNC('year', fe."date"::date) AS period_year,
    fev.id AS form_entry_value_id,
    ff.id AS form_field_id,
    ff.section_type,
    COALESCE(fev.coa_id, ff.coa_id) AS coa_id,
    coa.code AS account_code,
    coa.name AS account_name,
    coa.classification AS account_classification,
    at.id AS account_type_id,
    at.name AS account_type,
    COALESCE(fev.net_amount, 0) AS net_amount,
    COALESCE(fev.gst_amount, 0) AS gst_amount,
    COALESCE(fev.gross_amount, 0) AS gross_amount,
    fev.description,
    fev.business_percentage,
    cfv.form_id AS form_id,
    coa.account_tax_id AS tax_id,
    CASE
        WHEN at.name IN ('Asset', 'Expense') THEN 'DEBIT'
        WHEN at.name IN ('Liability', 'Equity', 'Revenue') THEN 'CREDIT'
        ELSE 'UNKNOWN'
    END AS normal_balance,
    CASE
        WHEN at.name IN ('Asset', 'Expense') AND COALESCE(fev.net_amount, 0) >= 0 THEN COALESCE(fev.net_amount, 0)
        WHEN at.name IN ('Liability', 'Equity', 'Revenue') AND COALESCE(fev.net_amount, 0) < 0 THEN ABS(COALESCE(fev.net_amount, 0))
        ELSE 0
    END AS debit_amount,
    CASE
        WHEN at.name IN ('Asset', 'Expense') AND COALESCE(fev.net_amount, 0) < 0 THEN ABS(COALESCE(fev.net_amount, 0))
        WHEN at.name IN ('Liability', 'Equity', 'Revenue') AND COALESCE(fev.net_amount, 0) >= 0 THEN COALESCE(fev.net_amount, 0)
        ELSE 0
    END AS credit_amount,
    CASE
        WHEN at.name IN ('Asset', 'Expense') THEN COALESCE(fev.net_amount, 0)
        WHEN at.name IN ('Liability', 'Equity', 'Revenue') THEN -COALESCE(fev.net_amount, 0)
        ELSE 0
    END AS entry_balance_variance,
    CASE
        WHEN at.name = 'Revenue' THEN COALESCE(fev.net_amount, 0)
        WHEN at.name = 'Expense' THEN -COALESCE(fev.net_amount, 0)
        ELSE NULL
    END AS profit_loss_amount,
    CASE
        WHEN coa.classification = 'Contra-Equity' THEN -COALESCE(fev.net_amount, 0)
        WHEN at.name IN ('Asset', 'Liability', 'Equity') THEN COALESCE(fev.net_amount, 0)
        ELSE NULL
    END AS balance_sheet_amount
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
            description,
            business_percentage
        FROM tbl_form_entry_value
        WHERE updated_at IS NULL
        ORDER BY entry_id,
            COALESCE(form_field_id::text, coa_id::text),
            created_at DESC
    ) fev ON fev.entry_id = fe.id
    LEFT JOIN tbl_form_field ff ON ff.id = fev.form_field_id
    JOIN tbl_chart_of_accounts coa ON coa.id = COALESCE(fev.coa_id, ff.coa_id)
    JOIN tbl_account_type at ON at.id = coa.account_type_id
WHERE fe.status = 'SUBMITTED'
    AND fe.deleted_at IS NULL
    AND coa.deleted_at IS NULL
    AND at.name IN ('Asset', 'Liability', 'Equity', 'Revenue', 'Expense')
    AND (
        (
            ff.id IS NOT NULL
            AND ff.deleted_at IS NULL
            AND ff.is_formula = FALSE
        )
        OR (
            ff.id IS NULL
            AND fev.coa_id IS NOT NULL
        )
    );



CREATE OR REPLACE VIEW vw_double_entry_entry_summary AS
SELECT practitioner_id,
    clinic_id,
    user_id,
    submitted_by,
    entry_id,
    date,
    entry_date,
    SUM(debit_amount) AS total_debit,
    SUM(credit_amount) AS total_credit,
    SUM(entry_balance_variance) AS variance,
    ABS(SUM(entry_balance_variance)) <= 0.01 AS is_balanced,
    COUNT(*) AS line_count
FROM vw_double_entry_line_items
GROUP BY practitioner_id,
    clinic_id,
    user_id,
    submitted_by,
    entry_id,
    date,
    entry_date;

CREATE OR REPLACE VIEW vw_balance_sheet_line_items AS
SELECT clinic_id,
    practitioner_id,
    user_id,
    submitted_by,
    entry_id,
    date,
    entry_date,
    period_month,
    period_year,
    form_field_id,
    section_type,
    coa_id,
    account_code,
    account_name,
    account_classification,
    account_type_id,
    account_type,
    net_amount,
    gross_amount,
    description,
    form_id,
    tax_id,
    balance_sheet_amount AS signed_amount
FROM vw_double_entry_line_items
WHERE account_type IN ('Asset', 'Liability', 'Equity');

CREATE OR REPLACE VIEW vw_balance_sheet_summary AS
SELECT practitioner_id,
    clinic_id,
    user_id,
    submitted_by,
    account_type,
    account_code,
    account_name,
    account_classification,
    coa_id,
    SUM(signed_amount) AS balance,
    COUNT(DISTINCT entry_id) AS entry_count,
    MAX(date) AS last_transaction_date
FROM vw_balance_sheet_line_items
GROUP BY practitioner_id,
    clinic_id,
    user_id,
    submitted_by,
    account_type,
    account_code,
    account_name,
    account_classification,
    coa_id
ORDER BY account_type,
    account_classification,
    account_code;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
DROP VIEW IF EXISTS vw_double_entry_entry_summary CASCADE;
DROP VIEW IF EXISTS vw_double_entry_line_items CASCADE;
-- +goose StatementEnd
