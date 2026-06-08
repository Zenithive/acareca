-- +goose Up
-- +goose StatementBegin

DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invoice_clinic_contact_type') THEN
        CREATE TYPE invoice_clinic_contact_type AS ENUM ('PHONE', 'WEBSITE');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'verification_token_status') THEN
        CREATE TYPE verification_token_status AS ENUM ('PENDING', 'USED', 'EXPIRED', 'RESENT');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS tbl_invoice_clinic (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    document_id   VARCHAR(255) NULL,
    clinic_name   VARCHAR(255) NOT NULL,
    description   TEXT NULL,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password      VARCHAR(100) NULL, 
    role          VARCHAR(50) NULL,  
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    abn           VARCHAR(11) NULL,
    acn           VARCHAR(9) NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NULL,
    deleted_at    TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_invoice_clinic_address (
    id         UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id  UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    address    TEXT NOT NULL,
    city       VARCHAR(100) NOT NULL,
    state      VARCHAR(50) NOT NULL,
    postcode   VARCHAR(20) NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_invoice_clinic_contacts (
    id           UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id    UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    contact_type invoice_clinic_contact_type NOT NULL,
    value        VARCHAR(255) NOT NULL,
    label        VARCHAR(100) NULL,
    is_primary   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NULL,
    deleted_at   TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_clinic_session (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id     UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL,
    user_agent    TEXT NULL,
    ip_address    VARCHAR(45) NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_clinic_verification_token (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE, 
    role       VARCHAR(50) NOT NULL,      
    status     verification_token_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tbl_clinic_contact_person (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    fname      VARCHAR(255) NOT NULL,
    lname      VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL,
    phone      VARCHAR(20) NULL,
    website    VARCHAR(255) NULL,
    abn        VARCHAR(20) NULL,
    note       TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_clinic_contact_person_address (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id    UUID NOT NULL REFERENCES tbl_clinic_contact_person(id) ON DELETE CASCADE,
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255) NULL,
    city          VARCHAR(100) NOT NULL,
    state         VARCHAR(100) NOT NULL,
    postal_code   VARCHAR(20) NOT NULL,
    country       VARCHAR(100) NOT NULL,
    is_primary    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_template (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    description TEXT NULL,
    html        BYTEA NOT NULL,
    css         BYTEA NOT NULL,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NULL,
    deleted_at  TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_template_setting (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id        UUID NOT NULL UNIQUE REFERENCES tbl_template(id) ON DELETE CASCADE,
    primary_color      VARCHAR(20) NOT NULL DEFAULT '#4247e7',
    accent_color       VARCHAR(20) NOT NULL DEFAULT '#000000',
    body_font_family   VARCHAR(100) NOT NULL DEFAULT 'Inter',
    header_font_family VARCHAR(100) NOT NULL DEFAULT 'Inter',
    is_logo            BOOLEAN NOT NULL DEFAULT FALSE,
    logo_id            UUID NULL, 
    letterhead_id      UUID NULL, 
    footer_id          UUID NULL, 
    terms_text         TEXT NULL,
    is_watermark       BOOLEAN NOT NULL DEFAULT FALSE,
    watermark_text     VARCHAR(100) NULL,
    is_tax             BOOLEAN NOT NULL DEFAULT TRUE,
    table_style        VARCHAR(100) NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NULL,
    deleted_at         TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_invoice (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id      UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    template_id    UUID NOT NULL REFERENCES tbl_template(id),
    contact_id     UUID NULL REFERENCES tbl_clinic_contact_person(id) ON DELETE SET NULL,
    name           VARCHAR(255) NOT NULL,
    invoice_number VARCHAR(100) NOT NULL,
    issue_date     DATE NOT NULL,
    due_date       DATE NULL,
    reference      VARCHAR(100) NULL,
    payment_method VARCHAR(50) NULL,
    tax_method     VARCHAR(50) NULL,
    subtotal       NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    tax_total      NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    grand_total    NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    status         VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'paid', 'overdue', 'void')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_invoice_item (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id   UUID NOT NULL REFERENCES tbl_invoice(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NULL,
    quantity     INT NOT NULL DEFAULT 1,
    unit_price   NUMERIC(12,2) NOT NULL,
    discount     NUMERIC(12,2) NULL,
    tax_rate     NUMERIC(5,2) NULL,
    tax_amount   NUMERIC(12,2) NULL,
    total_amount NUMERIC(12,2) NOT NULL,
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_inv_clinic_email ON tbl_invoice_clinic(email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_addr_lookup ON tbl_invoice_clinic_address(clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_cont_lookup ON tbl_invoice_clinic_contacts(clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_verification_token_entity ON tbl_clinic_verification_token(clinic_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_contact_person_primary_address ON tbl_clinic_contact_person_address (contact_id) WHERE is_primary = TRUE AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_person_clinic_id ON tbl_clinic_contact_person (clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_person_address_contact_id ON tbl_clinic_contact_person_address (contact_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_clinic_id ON tbl_invoice(clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoice_status ON tbl_invoice(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invoice_item_invoice ON tbl_invoice_item(invoice_id) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invoice_item;
DROP TABLE IF EXISTS tbl_invoice;
DROP TABLE IF EXISTS tbl_template_setting;
DROP TABLE IF EXISTS tbl_template;
DROP TABLE IF EXISTS tbl_clinic_contact_person_address;
DROP TABLE IF EXISTS tbl_clinic_contact_person;
DROP TABLE IF EXISTS tbl_clinic_verification_token;
DROP TABLE IF EXISTS tbl_clinic_session;
DROP TABLE IF EXISTS tbl_invoice_clinic_contacts;
DROP TABLE IF EXISTS tbl_invoice_clinic_address;
DROP TABLE IF EXISTS tbl_invoice_clinic;

DROP TYPE IF EXISTS verification_token_status;
DROP TYPE IF EXISTS invoice_clinic_contact_type;
-- +goose StatementEnd