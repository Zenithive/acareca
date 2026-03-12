-- +goose Up
-- +goose StatementBegin
CREATE TYPE clinic_contact_type_enum AS ENUM ('PHONE', 'EMAIL', 'WEBSITE', 'FAX');

CREATE TABLE IF NOT EXISTS tbl_clinic_contact (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    clinic_id       UUID NOT NULL REFERENCES tbl_clinic(id) ON DELETE CASCADE,
    contact_type    clinic_contact_type_enum NOT NULL,
    value           TEXT NOT NULL,
    label           TEXT,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_clinic_contact;
DROP TYPE IF EXISTS clinic_contact_type_enum;
-- +goose StatementEnd