-- +goose Up
-- +goose StatementBegin

CREATE TYPE practitioner_subscription_status AS ENUM ('ACTIVE', 'PAST_DUE', 'CANCELLED', 'PAUSED', 'EXPIRED', 'INACTIVE');

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'business_entity_type') THEN
        CREATE TYPE business_entity_type AS ENUM ('SOLE_TRADER', 'COMPANY', 'TRUST');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS tbl_practitioner (
    id         UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES tbl_user(id),
    verified   BOOLEAN NOT NULL DEFAULT FALSE,
    entity_name  VARCHAR(255),
    entity_type  business_entity_type NOT NULL DEFAULT 'SOLE_TRADER',
    address      TEXT,
    abn        VARCHAR(20),
    acn          VARCHAR(9),
    profession   VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_practitioner_setting (
    id              SERIAL PRIMARY KEY,
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id) ON DELETE CASCADE,
    timezone        VARCHAR(255) NOT NULL DEFAULT 'Australia/Sydney',
    logo            VARCHAR(255),
    color           VARCHAR(7) NOT NULL DEFAULT '#000000',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_practitioner_subscription (
    id              SERIAL PRIMARY KEY,
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id) ON DELETE CASCADE,
    subscription_id INTEGER NOT NULL,
    start_date      TIMESTAMPTZ NOT NULL,
    end_date        TIMESTAMPTZ NOT NULL,
    status          practitioner_subscription_status NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_practitioner_subscription;
DROP TYPE IF EXISTS practitioner_subscription_status;
DROP TABLE IF EXISTS tbl_practitioner_setting;
DROP TABLE IF EXISTS tbl_practitioner;

DROP TYPE IF EXISTS business_entity_type;
DROP TYPE IF EXISTS practitioner_subscription_status;
-- +goose StatementEnd