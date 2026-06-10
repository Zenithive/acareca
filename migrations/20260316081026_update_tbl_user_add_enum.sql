-- +goose Up
-- +goose StatementBegin

-- create enum
CREATE TYPE user_role AS ENUM (
    'ADMIN',
    'PRACTITIONER',
    'ACCOUNTANT'
);

-- add role column
ALTER TABLE tbl_user
ADD COLUMN role user_role NOT NULL DEFAULT 'PRACTITIONER';

-- remove old column
ALTER TABLE tbl_user
DROP COLUMN is_superadmin;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- add old column back
ALTER TABLE tbl_user
ADD COLUMN is_superadmin BOOLEAN NOT NULL DEFAULT FALSE;

-- remove role column
ALTER TABLE tbl_user
DROP COLUMN role;

-- drop enum
DROP TYPE IF EXISTS user_role;

-- +goose StatementEnd