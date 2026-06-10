-- +goose Up
-- +goose StatementBegin
ALTER TYPE section_type ADD VALUE 'EXPENSE_ENTRY';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd