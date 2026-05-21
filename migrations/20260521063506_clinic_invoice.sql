-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_invoice (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   uuid NOT NULL,
    template_id uuid NOT NULL,
    name        varchar(255) NOT NULL,
    invoice_number varchar(100) NOT NULL,
    issue_date  date NOT NULL,
    due_date    date,
    reference  varchar(100),
    payment_method    varchar(50),
    tax_method        varchar(50),
    subtotal    numeric(12, 2) NOT NULL DEFAULT 0,  -- store computed totals
    tax_total   numeric(12, 2) NOT NULL DEFAULT 0,
    grand_total numeric(12, 2) NOT NULL DEFAULT 0,
    status      varchar(20) NOT NULL DEFAULT 'draft'  -- draft/sent/paid/void
                CHECK (status IN ('draft', 'sent', 'paid', 'overdue', 'void')),
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW(),
    deleted_at  timestamptz
);

CREATE TABLE IF NOT EXISTS tbl_invoice_item (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id  uuid NOT NULL,
    name        varchar(255) NOT NULL,
    description text,
    quantity    int NOT NULL DEFAULT 1,
    unit_price  numeric(12, 2) NOT NULL,
    discount    numeric(12, 2),
    tax_rate    numeric(5, 2),
    tax_amount  numeric(12, 2),
    total_amount numeric(12, 2) NOT NULL,
    sort_order  int NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT NOW(),
    updated_at  timestamptz NOT NULL DEFAULT NOW(),
    deleted_at  timestamptz,

    CONSTRAINT fk_item_invoice FOREIGN KEY (invoice_id) REFERENCES tbl_invoice(id)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_invoice_clinic_id   ON tbl_invoice(clinic_id)  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoice_status      ON tbl_invoice(status)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoice_item_invoice ON tbl_invoice_item(invoice_id) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invoice_item;
DROP TABLE IF EXISTS tbl_invoice;
DROP INDEX IF EXISTS idx_invoice_clinic_id;
DROP INDEX IF EXISTS idx_invoice_status;
DROP INDEX IF EXISTS idx_invoice_item_invoice;
-- +goose StatementEnd
