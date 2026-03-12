-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_account_type (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tbl_account_type (name) VALUES
    ('Asset'),
    ('Liability'),
    ('Equity'),
    ('Revenue'),
    ('Expense')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS tbl_account_tax (
    id          SMALLSERIAL PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    rate        NUMERIC(5,2) NOT NULL DEFAULT 0,
    is_taxable  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tbl_account_tax (name, rate, is_taxable) VALUES
    ('GST on Income',     10.00, TRUE),
    ('GST on Expenses',   10.00, TRUE),
    ('GST Free Expenses',  0.00, FALSE),
    ('BAS Excluded',       0.00, FALSE),
    ('GST Free Income',    0.00, FALSE)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS tbl_chart_of_accounts (
    id               UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    practitioner_id   UUID NOT NULL,
    account_type_id  SMALLINT NOT NULL REFERENCES tbl_account_type(id),
    account_tax_id   SMALLINT NOT NULL REFERENCES tbl_account_tax(id),
    code             SMALLINT NOT NULL CHECK (code >= 100 AND code <= 9999),
    name             VARCHAR(255) NOT NULL,
    is_system        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT uq_chart_of_accounts_code_practitioner_id UNIQUE (code, practitioner_id)
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_chart_of_accounts;
DROP TABLE IF EXISTS tbl_account_tax;
DROP TABLE IF EXISTS tbl_account_type;
-- +goose StatementEnd
