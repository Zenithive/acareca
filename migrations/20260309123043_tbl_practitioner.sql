-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_practitioner (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES tbl_user(id),
    abn           VARCHAR(20),
    verified       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_practitioner_setting   (
    id            SERIAL PRIMARY KEY,
    practitioner_id    UUID NOT NULL REFERENCES tbl_practitioner(id),
    timezone      VARCHAR(255) NOT NULL DEFAULT 'Australia/Sydney',
    logo          VARCHAR(255),
    color         VARCHAR(7) NOT NULL DEFAULT '#000000',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_practitioner_setting;
DROP TABLE IF EXISTS tbl_practitioner;
-- +goose StatementEnd
