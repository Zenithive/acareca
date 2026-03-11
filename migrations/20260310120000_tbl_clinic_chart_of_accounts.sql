-- +goose Up
-- +goose StatementBegin
-- 1) Lookup: account categories (seeded)
CREATE TABLE IF NOT EXISTS tbl_account_type (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tbl_account_type (name, description) VALUES
    ('Asset', NULL),
    ('Liability', NULL),
    ('Equity', NULL),
    ('Revenue', NULL),
    ('Expense', NULL)
ON CONFLICT (name) DO NOTHING;

-- 2) Lookup: GST tax treatment codes (seeded)
CREATE TABLE IF NOT EXISTS tbl_account_tax (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    rate        NUMERIC(5,2) NOT NULL DEFAULT 0,
    bas_field   VARCHAR(10),
    is_taxable  BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tbl_account_tax (name, rate, bas_field, is_taxable, description) VALUES
    ('GST on Income',     10.00, 'G1',  TRUE,  NULL),
    ('GST on Expenses',   10.00, 'G3',  TRUE,  NULL),
    ('GST Free Expenses',  0.00, '1A',  FALSE, NULL),
    ('BAS Excluded',       0.00, 'G11', FALSE, NULL),
    ('GST Free Income',    0.00, '1B',  FALSE, NULL)
ON CONFLICT (name) DO NOTHING;

-- 3) Chart of Accounts (global; created_by = practitioner id; unique per code+created_by)
CREATE TABLE IF NOT EXISTS tbl_chart_of_accounts (
    id               UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    created_by       UUID NOT NULL,
    account_type_id  SMALLINT NOT NULL REFERENCES tbl_account_type(id),
    account_tax_id   SMALLINT NOT NULL REFERENCES tbl_account_tax(id),
    code             VARCHAR(10) NOT NULL,
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    is_system        BOOLEAN NOT NULL DEFAULT FALSE,
    system_provider  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT uq_chart_of_accounts_code_created_by UNIQUE (code, created_by)
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_chart_of_accounts;
DROP TABLE IF EXISTS tbl_account_tax;
DROP TABLE IF EXISTS tbl_account_type;
-- +goose StatementEnd
