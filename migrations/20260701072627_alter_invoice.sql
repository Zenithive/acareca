-- +goose Up
-- +goose StatementBegin
CREATE TYPE invoice_method AS ENUM ('SFA_CLINIC_COLLECTS', 'SFA_DENTIST_COLLECTS', 'INDEPENDENT_CONTRACTOR');

ALTER TABLE tbl_invoice 
    ADD COLUMN invoice_method invoice_method NOT NULL DEFAULT 'SFA_CLINIC_COLLECTS';

ALTER TYPE invoice_section RENAME TO invoice_section_v2;
CREATE TYPE invoice_section_v2 AS ENUM (
    'CALCULATION_STATEMENT',
    'TAX_INVOICE',
    'REMITTANCE_INVOICE',
    'RCTI'
);

ALTER TABLE tbl_map_invoice_section
ALTER COLUMN invoice_section TYPE invoice_section
USING (
    CASE invoice_section::text
        WHEN 'SFA_INVOICE' THEN 'TAX_INVOICE'
        ELSE invoice_section::text
    END
)::invoice_section;

ALTER TABLE tbl_map_invoice_section
    DROP COLUMN invoice_section,
    ADD COLUMN invoice_section invoice_section_v2;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_invoice 
    DROP COLUMN invoice_method;

DROP TYPE IF EXISTS invoice_section_v2;
CREATE TYPE invoice_section AS ENUM (
    'CALCULATION_STATEMENT',
    'TAX_INVOICE',
    'REMITTANCE_INVOICE',
    'RCTI'
);

ALTER TABLE tbl_map_invoice_section
    DROP COLUMN invoice_section,
    ADD COLUMN invoice_section invoice_section NOT NULL;
-- +goose StatementEnd
