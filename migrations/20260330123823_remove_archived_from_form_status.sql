-- +goose Up
-- +goose StatementBegin
-- 1. CLEANUP DATA: Cast to ::TEXT to bypass Enum validation during the check
UPDATE tbl_form 
SET status = 'DRAFT'::form_status 
WHERE status::text = 'ARCHIVED';
-- +goose StatementEnd

-- +goose StatementBegin
-- 2. CREATE NEW TYPE without ARCHIVED
CREATE TYPE form_status_new AS ENUM ('DRAFT', 'PUBLISHED');

-- 3. ALTER COLUMN
ALTER TABLE tbl_form 
    ALTER COLUMN status TYPE form_status_new 
    USING status::text::form_status_new;

-- 4. SWAP NAMES
DROP TYPE form_status;
ALTER TYPE form_status_new RENAME TO form_status;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TYPE form_status RENAME TO form_status_old;
CREATE TYPE form_status AS ENUM ('DRAFT', 'PUBLISHED', 'ARCHIVED');
ALTER TABLE tbl_form 
    ALTER COLUMN status TYPE form_status 
    USING status::text::form_status;
DROP TYPE form_status_old;
-- +goose StatementEnd