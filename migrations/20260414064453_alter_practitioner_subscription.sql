-- +goose Up
-- +goose StatementBegin
ALTER TYPE practitioner_subscription_status ADD VALUE 'INACTIVE';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd