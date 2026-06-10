-- +goose Up
-- +goose StatementBegin

CREATE TYPE section_type AS ENUM ('COLLECTION', 'COST', 'OTHER_COST', 'EXPENSE_ENTRY');

CREATE TYPE payment_responsibility AS ENUM ('OWNER', 'CLINIC');

CREATE TYPE tax_type AS ENUM ('INCLUSIVE', 'EXCLUSIVE', 'MANUAL');

CREATE TYPE entry_status AS ENUM ('DRAFT', 'SUBMITTED');

CREATE TABLE IF NOT EXISTS tbl_form_field (
    id                     UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_version_id        UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    label                  VARCHAR(255) NOT NULL,
    section_type           section_type NULL,
    payment_responsibility payment_responsibility NULL,
    tax_type               tax_type NULL,              
    coa_id                 UUID NULL REFERENCES tbl_chart_of_accounts(id) ON DELETE CASCADE,
    sort_order             INTEGER NOT NULL DEFAULT 0,
    field_key              VARCHAR(5) NOT NULL DEFAULT '',
    slug                   VARCHAR(100) NULL,
    is_computed            BOOLEAN NOT NULL DEFAULT FALSE,
    is_formula             BOOLEAN NOT NULL DEFAULT FALSE,
    is_highlighted         BOOLEAN NOT NULL DEFAULT FALSE,
    business_use           DOUBLE PRECISION NULL,
    amount                 DOUBLE PRECISION NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at             TIMESTAMPTZ DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS tbl_form_entry (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_version_id UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    clinic_id       UUID NOT NULL,
    date            DATE NULL,
    submitted_by    UUID NULL,
    submitted_at    TIMESTAMPTZ NULL,
    status          entry_status NOT NULL DEFAULT 'DRAFT',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS tbl_form_entry_value (
    id                  UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    entry_id            UUID NOT NULL REFERENCES tbl_form_entry(id) ON DELETE CASCADE,
    form_field_id       UUID NULL REFERENCES tbl_form_field(id) ON DELETE CASCADE,
    description         TEXT NULL,
    coa_id              UUID NULL REFERENCES tbl_chart_of_accounts(id) ON DELETE SET NULL,
    net_amount          NUMERIC(10, 2) NULL,
    gst_amount          NUMERIC(10, 2) NULL,
    gross_amount        NUMERIC(10, 2) NULL,
    date                DATE NULL,
    business_percentage NUMERIC(5, 2) NULL DEFAULT 100.00,
    notes               TEXT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NULL, 
    deleted_at          TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_tbl_form_field_form_version_id ON tbl_form_field(form_version_id);
CREATE INDEX IF NOT EXISTS idx_tbl_form_field_sort_order ON tbl_form_field(form_version_id, sort_order);

CREATE UNIQUE INDEX uniq_form_field_key ON tbl_form_field(form_version_id, field_key);

CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_form_version_id ON tbl_form_entry(form_version_id);
CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_clinic_id ON tbl_form_entry(clinic_id);

CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_value_entry_id ON tbl_form_entry_value(entry_id);
CREATE INDEX IF NOT EXISTS idx_form_entry_value_deleted_at ON tbl_form_entry_value (deleted_at) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_form_entry_value;
DROP TABLE IF EXISTS tbl_form_entry;
DROP TABLE IF EXISTS tbl_form_field;

DROP TYPE IF EXISTS entry_status;
DROP TYPE IF EXISTS tax_type;
DROP TYPE IF EXISTS payment_responsibility;
DROP TYPE IF EXISTS section_type;
-- +goose StatementEnd