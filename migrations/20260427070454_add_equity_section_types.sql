-- +goose Up
-- +goose StatementBegin

-- Add new section types for equity transactions
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_WITHDRAWAL';
ALTER TYPE section_type ADD VALUE IF NOT EXISTS 'EQUITY_CONTRIBUTION';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin


-- +goose StatementEnd
