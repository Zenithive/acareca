-- +goose Up
-- +goose StatementBegin

CREATE TYPE invoice_section AS ENUM (
    'CALCULATION_STATEMENT',
    'SFA_INVOICE',
    'REMITTANCE_INVOICE'
);

CREATE TYPE invoice_frequency AS ENUM (
    'DAILY',
    'WEEKLY',
    'MONTHLY',
    'YEARLY'
);

ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS reference,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS tax_method,
    DROP COLUMN IF EXISTS subtotal,
    DROP COLUMN IF EXISTS tax_total,
    DROP COLUMN IF EXISTS grand_total;

ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS billing_period VARCHAR(200),
    ADD COLUMN IF NOT EXISTS invoice_frequency invoice_frequency;

ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS invoice_id,
    DROP COLUMN IF EXISTS discount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_amount;

CREATE TABLE IF NOT EXISTS tbl_map_invoice_section (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES tbl_invoice(id),
    invoice_section invoice_section NOT NULL,
    document_number VARCHAR(100) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    UNIQUE (invoice_id, invoice_section)
);

ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS bas_code VARCHAR(20),
    ADD COLUMN IF NOT EXISTS invoice_section_id UUID,
    ADD COLUMN IF NOT EXISTS invoice_frequency invoice_frequency;

ALTER TABLE tbl_invoice_item
    ADD CONSTRAINT fk_invoice_item_invoice_section
    FOREIGN KEY (invoice_section_id)
    REFERENCES tbl_map_invoice_section(id);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_invoice_item
    DROP CONSTRAINT IF EXISTS fk_invoice_item_invoice_section;

ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS bas_code,
    DROP COLUMN IF EXISTS invoice_section_id,
    DROP COLUMN IF EXISTS invoice_frequency;

DROP TABLE IF EXISTS tbl_map_invoice_section;

ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS billing_period,
    DROP COLUMN IF EXISTS invoice_frequency;

ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS reference VARCHAR(255),
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS tax_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS subtotal NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS tax_total NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS grand_total NUMERIC(12,2);

ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS invoice_id UUID,
    ADD COLUMN IF NOT EXISTS discount NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(12,2);

DROP TYPE IF EXISTS invoice_frequency;
DROP TYPE IF EXISTS invoice_section;

-- +goose StatementEnd