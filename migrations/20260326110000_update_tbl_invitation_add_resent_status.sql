-- +goose Up
-- +goose StatementBegin
ALTER TYPE invitation_status ADD VALUE IF NOT EXISTS 'RESENT';
-- +goose StatementEnd

-- +goose Down
-- Note: PostgreSQL does not support removing values from an ENUM. 
-- To undo this, one would typically have to recreate the type and table.