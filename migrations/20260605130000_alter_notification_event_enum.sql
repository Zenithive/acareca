-- +goose Up
-- +goose StatementBegin

ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'new.transaction';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'accountant.activity.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'system.activity.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'system.error.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'system.warning.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'billing.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'subscription.alert';
ALTER TYPE enum_notification_event ADD VALUE IF NOT EXISTS 'user.registration.alert';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- PostgreSQL does not support removing enum values in-place.
-- No-op down migration to keep goose reversible behavior.
-- +goose StatementEnd
