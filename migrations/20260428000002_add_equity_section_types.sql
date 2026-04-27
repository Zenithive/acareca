-- +goose Up
-- +goose StatementBegin

-- Add new section types for equity transactions
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_WITHDRAWAL';
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_CONTRIBUTION';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: PostgreSQL does not support removing enum values directly
-- If rollback is needed, you would need to:
-- 1. Create a new enum without these values
-- 2. Alter all columns to use the new enum
-- 3. Drop the old enum
-- This is complex and rarely needed, so we leave it as a manual process

-- +goose StatementEnd
