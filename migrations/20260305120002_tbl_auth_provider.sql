-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_auth_provider (
    id                UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),

    user_id           UUID NOT NULL REFERENCES tbl_user(id),

    provider          VARCHAR(50) NOT NULL,

    access_token      TEXT,
    refresh_token     TEXT,
    token_expires_at  TIMESTAMPTZ,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,

    CONSTRAINT uq_auth_provider_user_provider UNIQUE (user_id, provider)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_auth_provider;
-- +goose StatementEnd
