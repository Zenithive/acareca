-- +goose Up
-- +goose StatementBegin
CREATE TABLE tbl_user (
    id            VARCHAR(40) PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password      VARCHAR(100) NOT NULL,                             -- Argon2id; NULL = invited-only
    first_name    VARCHAR(255) NOT NULL,
    last_name     VARCHAR(255) NOT NULL,
    phone         VARCHAR(20),                             -- E.164 format
    is_superadmin BOOLEAN NOT NULL DEFAULT FALSE,   -- Platform owner; no practice context
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ                       -- Soft delete
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tbl_user;
-- +goose StatementEnd
feat: added user module with basic auth functionality

Enhance project setup by adding new dependencies in go.mod, updating README for better clarity on project structure and setup, and refactoring database connection logic for improved concurrency handling.