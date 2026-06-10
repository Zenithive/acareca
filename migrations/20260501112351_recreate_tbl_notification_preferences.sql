-- +goose Up
-- +goose StatementBegin

-- 1. Clear old table if it exists
DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TYPE IF EXISTS enum_notification_event;

-- 2. Create ENUM for allowed events 
-- This ensures strict validation at the database level.
CREATE TYPE enum_notification_event AS ENUM (
    'NEW_TRANSACTION',
    'ACCOUNTANT_ACTIVITY_ALERT',
    'SYSTEM_ACTIVITY_ALERT',
    'BAS_REMINDER',
    'ENTRY_REVIEW_ALERT',
    'WEEKLY_SUMMARY',
    'MARKETING_UPDATE'
);

-- 3. Create the clean schema
CREATE TABLE tbl_notification_preferences (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Owner of the settings
    user_id         UUID         NOT NULL,
    
    -- Actor details
    entity_id       UUID         NOT NULL, -- actor_id
    entity_type     VARCHAR(64)  NOT NULL, -- role (PRACTITIONER, ACCOUNTANT, ADMIN)

    -- Event Type restricted by the ALL CAPS ENUM
    event_type      enum_notification_event NOT NULL,

    -- Flexible channels map (e.g., {"push": true, "in_app": true})
    channels        JSONB        NOT NULL DEFAULT '{"in_app": true}',

    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ, -- Support for soft deletes

    -- Ensure a unique preference record per user/entity/event
    UNIQUE (user_id, entity_id, event_type)
);

-- Index for lookup performance (ignoring soft-deleted entries)
CREATE INDEX idx_notif_prefs_active ON tbl_notification_preferences (user_id, event_type) 
WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TYPE IF EXISTS enum_notification_event;
-- +goose StatementEnd