-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_accountant (
    id          UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES tbl_user(id),
    license_no  VARCHAR(50),
    verified    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_accountant_setting (
    id            SERIAL PRIMARY KEY,
    accountant_id UUID NOT NULL REFERENCES tbl_accountant(id),
    timezone      VARCHAR(255) NOT NULL DEFAULT 'Australia/Sydney',
    settings   JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_accountant_setting;
DROP TABLE IF EXISTS tbl_accountant;
-- +goose StatementEnd