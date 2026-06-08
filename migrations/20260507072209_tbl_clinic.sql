-- +goose Up
-- +goose StatementBegin

CREATE TYPE clinic_contact_type_enum AS ENUM ('PHONE', 'EMAIL', 'WEBSITE', 'FAX');

CREATE TYPE account_classification AS ENUM (
    'Current Asset', 
    'Non-Current Asset', 
    'Contra-Asset', 
    'Current Liability', 
    'Non-Current Liability', 
    'Equity', 
    'Contra-Equity', 
    'Operating Revenue', 
    'Other Revenue', 
    'Direct Costs', 
    'Operating Expense'
);

CREATE TABLE IF NOT EXISTS tbl_clinic (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(), 
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id),
    document_id     UUID NULL,
    name            VARCHAR(150) NOT NULL, 
    abn             VARCHAR(11), 
    description     TEXT, 
    is_active       BOOLEAN NOT NULL DEFAULT TRUE, 
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(), 
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_tbl_clinic_entity_id ON tbl_clinic(entity_id);

CREATE TABLE IF NOT EXISTS tbl_clinic_address (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    clinic_id       UUID NOT NULL REFERENCES tbl_clinic(id) ON DELETE CASCADE,
    address         TEXT,
    city            TEXT,
    state           TEXT,
    postcode        VARCHAR(4),
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_clinic_address_deleted_at ON tbl_clinic_address (deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tbl_clinic_contact (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    clinic_id       UUID NOT NULL REFERENCES tbl_clinic(id) ON DELETE CASCADE,
    contact_type    clinic_contact_type_enum NOT NULL,
    value           TEXT NOT NULL,
    label           TEXT,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_clinic_contact_deleted_at ON tbl_clinic_contact (deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tbl_financial_settings (
    id                  UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(), 
    clinic_id           UUID NULL REFERENCES tbl_clinic(id) ON DELETE SET NULL, 
    practitioner_id     UUID NOT NULL REFERENCES tbl_practitioner(id),
    financial_year_id   UUID NOT NULL REFERENCES tbl_financial_year(id),
    lock_date           DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
    deleted_at          TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_fin_settings_deleted_at ON tbl_financial_settings (deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tbl_account_type (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tbl_account_tax (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    rate        NUMERIC(5,2) NOT NULL DEFAULT 0,
    is_taxable  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tbl_chart_of_accounts (
    id              UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    practitioner_id UUID NOT NULL,
    account_type_id SMALLINT NOT NULL REFERENCES tbl_account_type(id),  
    account_tax_id  SMALLINT NOT NULL REFERENCES tbl_account_tax(id),
    code            SMALLINT NOT NULL CHECK (code >= 100 AND code <= 9999),
    name            VARCHAR(255) NOT NULL,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    key             VARCHAR(255) NOT NULL;
    classification  account_classification NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_chart_of_accounts_code_practitioner_id
ON tbl_chart_of_accounts (code, practitioner_id)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_chart_of_accounts_key_practitioner_id
ON tbl_chart_of_accounts (key, practitioner_id)
WHERE deleted_at IS NULL;

INSERT INTO tbl_account_type (name) VALUES
    ('Asset'), ('Liability'), ('Equity'), ('Revenue'), ('Expense')
ON CONFLICT (name) DO NOTHING;

INSERT INTO tbl_account_tax (name, rate, is_taxable) VALUES
    ('GST on Income',     10.00, TRUE),
    ('GST on Expenses',   10.00, TRUE),
    ('GST Free Expenses',  0.00, FALSE),
    ('BAS Excluded',       0.00, FALSE),
    ('GST Free Income',    0.00, FALSE)
ON CONFLICT (name) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_chart_of_accounts_key_practitioner_id;
DROP INDEX IF EXISTS uq_chart_of_accounts_code_practitioner_id;
DROP INDEX IF EXISTS idx_fin_settings_deleted_at;
DROP INDEX IF EXISTS idx_clinic_contact_deleted_at;
DROP INDEX IF EXISTS idx_clinic_address_deleted_at;
DROP INDEX IF EXISTS idx_tbl_clinic_entity_id;
DROP TABLE IF EXISTS tbl_chart_of_accounts;
DROP TABLE IF EXISTS tbl_account_tax;
DROP TABLE IF EXISTS tbl_account_type;
DROP TABLE IF EXISTS tbl_financial_settings;
DROP TABLE IF EXISTS tbl_clinic_contact;
DROP TABLE IF EXISTS tbl_clinic_address;
DROP TABLE IF EXISTS tbl_clinic;

DROP TYPE IF EXISTS account_classification;
DROP TYPE IF EXISTS clinic_contact_type_enum;
-- +goose StatementEnd