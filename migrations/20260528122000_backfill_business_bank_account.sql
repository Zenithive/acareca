-- +goose Up
-- +goose StatementBegin
INSERT INTO tbl_chart_of_accounts (
    practitioner_id,
    account_type_id,
    account_tax_id,
    code,
    name,
    key,
    is_system,
    classification
)
SELECT p.id,
    at.id,
    tax.id,
    600,
    'Business Bank Account',
    'business_bank_account',
    TRUE,
    'Current Asset'::account_classification
FROM tbl_practitioner p
JOIN tbl_account_type at ON at.name = 'Asset'
JOIN tbl_account_tax tax ON tax.name = 'BAS Excluded'
WHERE NOT EXISTS (
    SELECT 1
    FROM tbl_chart_of_accounts coa
    WHERE coa.practitioner_id = p.id
        AND coa.code = 600
        AND coa.deleted_at IS NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM tbl_chart_of_accounts
WHERE code = 600
    AND key = 'business_bank_account'
    AND is_system = TRUE
    AND NOT EXISTS (
        SELECT 1
        FROM tbl_form_entry_value fev
        WHERE fev.coa_id = tbl_chart_of_accounts.id
            AND fev.deleted_at IS NULL
    );
-- +goose StatementEnd
