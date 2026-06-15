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

CREATE TYPE tax_method AS ENUM (
    'INCLUSIVE',
    'EXCLUSIVE',
    'NO_TAX'
);

ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS invoice_number,
    DROP COLUMN IF EXISTS reference,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS tax_method,
    DROP COLUMN IF EXISTS subtotal,
    DROP COLUMN IF EXISTS tax_total,
    DROP COLUMN IF EXISTS grand_total;

ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS billing_period_from DATE,
    ADD COLUMN IF NOT EXISTS billing_period_to DATE,
    ADD COLUMN IF NOT EXISTS invoice_frequency invoice_frequency;

-- Remove old columns from invoice items but KEEP invoice_id
ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS discount,
    DROP COLUMN IF EXISTS tax_rate,
    DROP COLUMN IF EXISTS tax_amount;

-- Create invoice sections mapping table
CREATE TABLE IF NOT EXISTS tbl_map_invoice_section (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES tbl_invoice(id) ON DELETE CASCADE,
    invoice_section invoice_section NOT NULL,
    document_number VARCHAR(100) NOT NULL,
    tax_method tax_method DEFAULT 'NO_TAX',
    tax_rate NUMERIC(5,2) DEFAULT 0.00,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    UNIQUE (invoice_id, invoice_section)
);

-- Add new columns to invoice items
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS bas_code VARCHAR(20),
    ADD COLUMN IF NOT EXISTS invoice_section_id UUID,
    ADD COLUMN IF NOT EXISTS entry_type VARCHAR(50);

-- Add foreign key constraint for invoice section
ALTER TABLE tbl_invoice_item
    ADD CONSTRAINT fk_invoice_item_invoice_section
    FOREIGN KEY (invoice_section_id)
    REFERENCES tbl_map_invoice_section(id)
    ON DELETE SET NULL;

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_invoice_section_invoice_id
    ON tbl_map_invoice_section(invoice_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_item_section_id
    ON tbl_invoice_item(invoice_section_id)
    WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Drop indexes
DROP INDEX IF EXISTS idx_invoice_item_section_id;
DROP INDEX IF EXISTS idx_invoice_section_invoice_id;

-- Drop foreign key constraint
ALTER TABLE tbl_invoice_item
    DROP CONSTRAINT IF EXISTS fk_invoice_item_invoice_section;

-- Remove new columns
ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS bas_code,
    DROP COLUMN IF EXISTS invoice_section_id,
    DROP COLUMN IF EXISTS entry_type,
    DROP COLUMN IF EXISTS amount;

-- Drop the mapping table
DROP TABLE IF EXISTS tbl_map_invoice_section;

-- Remove new invoice columns
ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS contact_id,
    DROP COLUMN IF EXISTS billing_period_from,
    DROP COLUMN IF EXISTS billing_period_to,
    DROP COLUMN IF EXISTS invoice_frequency;

-- Restore old invoice columns
ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS invoice_number VARCHAR(100),
    ADD COLUMN IF NOT EXISTS reference VARCHAR(255),
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS tax_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS subtotal NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS tax_total NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS grand_total NUMERIC(12,2);

-- Restore old invoice_item columns
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS discount NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(5,2),
    ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(12,2);

-- Drop custom types
DROP TYPE IF EXISTS invoice_frequency;
DROP TYPE IF EXISTS invoice_section;
DROP TYPE IF EXISTS tax_method;

-- +goose StatementEnd