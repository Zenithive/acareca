-- +goose Up
-- +goose StatementBegin

CREATE TYPE form_status AS ENUM ('DRAFT', 'PUBLISHED');

CREATE TYPE calculation_method AS ENUM ('INDEPENDENT_CONTRACTOR', 'SERVICE_FEE', 'EXPENSE_ENTRY');

CREATE TABLE IF NOT EXISTS tbl_form (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    clinic_id       UUID NULL, 
    name            VARCHAR(40) NOT NULL,
    description     TEXT,
    status          form_status NOT NULL,
    method          calculation_method NOT NULL,
    owner_share     INTEGER NOT NULL,
    clinic_share    INTEGER NOT NULL,
    super_component DECIMAL(5, 2) NULL CONSTRAINT chk_super_component_range CHECK (super_component IS NULL OR (super_component >= 0 AND super_component <= 100)),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS tbl_custom_form_version (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_id         UUID NOT NULL REFERENCES tbl_form(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    is_active       BOOLEAN NOT NULL,
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_custom_form_practitioner_id ON tbl_custom_form_version(practitioner_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_custom_form_version;
DROP TABLE IF EXISTS tbl_form;

DROP TYPE IF EXISTS calculation_method;
DROP TYPE IF EXISTS form_status;
-- +goose StatementEnd