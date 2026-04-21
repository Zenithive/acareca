-- +goose Up
-- +goose StatementBegin

-- Add 'ACCOUNTANT' to the invite_entity_type enum
ALTER TYPE invite_entity_type ADD VALUE IF NOT EXISTS 'ACCOUNTANT';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: PostgreSQL does not support removing enum values directly.
-- If you need to rollback, you would need to:
-- 1. Create a new enum without 'ACCOUNTANT'
-- 2. Alter the table to use the new enum
-- 3. Drop the old enum
-- This is complex and risky, so we're leaving the down migration empty.
-- Manual intervention would be required for a true rollback.

-- +goose StatementEnd
