-- +goose Up
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TYPE IF EXISTS enum_notification_event;

CREATE TYPE enum_notification_event AS ENUM (
    'new.transaction',
    'accountant.activity.alert',
    'system.activity.alert'
);

CREATE TABLE tbl_notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    entity_id UUID NOT NULL,
    entity_type VARCHAR(64) NOT NULL,

    event_type enum_notification_event NOT NULL,

    channels JSONB NOT NULL DEFAULT jsonb_build_object(
        'in_app', true,
        'email', true,
        'push', true
    ),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    UNIQUE (user_id, entity_id, event_type)
);

CREATE INDEX idx_notif_prefs_active
ON tbl_notification_preferences (user_id, event_type)
WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TYPE IF EXISTS enum_notification_event;

-- +goose StatementEnd