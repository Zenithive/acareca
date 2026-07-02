-- +goose Up
-- +goose StatementBegin

CREATE TYPE contact_person_role AS ENUM (
    'DENTIST',
    'PATIENT'
);

ALTER TABLE tbl_clinic_contact_person
    ADD COLUMN IF NOT EXISTS role contact_person_role DEFAULT 'PATIENT';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TYPE IF EXISTS contact_person_role;

ALTER TABLE tbl_clinic_contact_person
    DROP COLUMN IF EXISTS role;

-- +goose StatementEnd