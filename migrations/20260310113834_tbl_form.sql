-- +goose Up
-- +goose StatementBegin

CREATE TYPE form_status AS ENUM ('DRAFT', 'PUBLISHED', 'ARCHIVED');

CREATE TYPE calculation_method AS ENUM('INDEPENDENT_CONTRACTOR', 'SERVICE_FEE');

CREATE TABLE IF NOT EXISTS tbl_form(
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    clinic_id UUID NOT NULL,
    name VARCHAR(40) NOT NULL,
    description TEXT,
    status form_status NOT NULL,
    method calculation_method NOT NULL,
    owner_share INTEGER NOT NULL,
    clinic_share INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_custom_form_version(
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_id UUID NOT NULL REFERENCES tbl_form(id),
    version INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL,

    practitioner_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);



-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_custom_form_version;
DROP TABLE IF EXISTS tbl_form;
DROP TYPE IF EXISTS form_status;
DROP TYPE IF EXISTS calculation_method;
-- +goose StatementEnd
