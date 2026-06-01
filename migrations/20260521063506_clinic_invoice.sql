-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_invoice (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       UUID NOT NULL,
    template_id     UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    invoice_number  VARCHAR(100) NOT NULL,
    issue_date      DATE NOT NULL,
    due_date        DATE,
    reference       VARCHAR(100),
    payment_method  VARCHAR(50),
    tax_method      VARCHAR(50),

    subtotal        NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax_total       NUMERIC(12,2) NOT NULL DEFAULT 0,
    grand_total     NUMERIC(12,2) NOT NULL DEFAULT 0,

    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'sent', 'paid', 'overdue', 'void')),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);


CREATE TABLE IF NOT EXISTS tbl_invoice_item (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id    UUID NOT NULL,

    name          VARCHAR(255) NOT NULL,
    description   TEXT,

    quantity      INT NOT NULL DEFAULT 1,
    unit_price    NUMERIC(12,2) NOT NULL,
    discount      NUMERIC(12,2),
    tax_rate      NUMERIC(5,2),
    tax_amount    NUMERIC(12,2),
    total_amount  NUMERIC(12,2) NOT NULL,

    sort_order    INT NOT NULL DEFAULT 0,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT fk_item_invoice
        FOREIGN KEY (invoice_id)
        REFERENCES tbl_invoice(id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_invoice_clinic_id
    ON tbl_invoice(clinic_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_status
    ON tbl_invoice(status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_item_invoice
    ON tbl_invoice_item(invoice_id)
    WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_invoice_item;
DROP TABLE IF EXISTS tbl_invoice;

-- +goose StatementEnd