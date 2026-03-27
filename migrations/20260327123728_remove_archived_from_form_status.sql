-- +goose Up
-- +goose StatementBegin

-- 1. Create a temporary type without 'ARCHIVED'
CREATE TYPE form_status_new AS ENUM ('DRAFT', 'PUBLISHED');

-- 2. Update existing rows: If any are 'ARCHIVED', move them to 'DRAFT' (or your preferred fallback)
UPDATE tbl_form SET status = 'DRAFT' WHERE status = 'ARCHIVED';

-- 3. Change the column type to use the new ENUM
-- We use USING to explicitly cast the old values to the new type
ALTER TABLE tbl_form 
    ALTER COLUMN status TYPE form_status_new 
    USING status::text::form_status_new;

-- 4. Drop the old type and rename the new one
DROP TYPE form_status;
ALTER TYPE form_status_new RENAME TO form_status;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- To go back, we just add 'ARCHIVED' back into the type
-- Note: Postgres does not allow adding values to ENUMs inside a transaction block easily,
-- but for a migration rollback, this is the standard approach.

ALTER TYPE form_status ADD VALUE 'ARCHIVED';

-- +goose StatementEnd