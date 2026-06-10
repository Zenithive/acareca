-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_clinic_contact_person (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  UUID NOT NULL,
    fname      VARCHAR(255) NOT NULL,
    lname      VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL,
    phone      VARCHAR(20),
    website    VARCHAR(255),
    abn        VARCHAR(20),
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_clinic_contact_person_address (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id    UUID NOT NULL,
    address_line1 VARCHAR(255) NOT NULL,
    address_line2 VARCHAR(255),
    city          VARCHAR(100) NOT NULL,
    state         VARCHAR(100) NOT NULL,
    postal_code   VARCHAR(20)  NOT NULL,
    country       VARCHAR(100) NOT NULL,
    is_primary    BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT fk_address_contact
        FOREIGN KEY (contact_id)
        REFERENCES tbl_clinic_contact_person(id)
);

-- Only one primary address allowed per contact
CREATE UNIQUE INDEX IF NOT EXISTS uq_contact_person_primary_address
    ON tbl_clinic_contact_person_address (contact_id)
    WHERE is_primary = TRUE AND deleted_at IS NULL;

-- Common query indexes
CREATE INDEX IF NOT EXISTS idx_contact_person_clinic_id
    ON tbl_clinic_contact_person (clinic_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_contact_person_address_contact_id
    ON tbl_clinic_contact_person_address (contact_id)
    WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_clinic_contact_person_address;
DROP TABLE IF EXISTS tbl_clinic_contact_person;

-- +goose StatementEnd