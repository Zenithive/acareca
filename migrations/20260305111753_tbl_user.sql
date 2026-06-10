-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS tbl_user (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password      VARCHAR(100),                                      -- Argon2id; NULL = OAuth-only user
    first_name    VARCHAR(255) NOT NULL,
    last_name     VARCHAR(255) NOT NULL,
    phone         VARCHAR(20),                             -- E.164 format
    avatar_url TEXT,
    is_superadmin BOOLEAN NOT NULL DEFAULT FALSE,   -- Platform owner; no practice context
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ                       -- Soft delete
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_user;
-- +goose StatementEnd
