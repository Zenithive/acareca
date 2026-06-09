-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create role enum
CREATE TYPE user_role AS ENUM (
    'ADMIN',
    'PRACTITIONER',
    'ACCOUNTANT'
);

-- Create table with final unified schema structures
CREATE TABLE IF NOT EXISTS tbl_user (
    id          UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    email       VARCHAR(255) UNIQUE NOT NULL,
    password    VARCHAR(100),                                      -- Argon2id; NULL = OAuth-only user
    first_name  VARCHAR(255) NOT NULL,
    last_name   VARCHAR(255) NOT NULL,
    phone       VARCHAR(20),                                       -- E.164 format
    role        user_role NOT NULL DEFAULT 'PRACTITIONER',
    document_id UUID NULL REFERENCES tbl_document(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ                                        -- Soft delete
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_user;
DROP TYPE IF EXISTS user_role;
-- +goose StatementEnd