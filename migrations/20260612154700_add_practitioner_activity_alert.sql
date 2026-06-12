-- +goose Up
-- +goose StatementBegin
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'practitioner.activity.alert';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Note: PostgreSQL does not support removing enum values
-- The value 'practitioner.activity.alert' will remain in the enum
-- +goose StatementEnd
