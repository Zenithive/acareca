-- +goose Up
-- +goose StatementBegin

DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'enum_notification_status') THEN
        CREATE TYPE enum_notification_status AS ENUM ('UNREAD', 'READ', 'DISMISSED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'enum_delivery_status') THEN
        CREATE TYPE enum_delivery_status AS ENUM ('PENDING', 'DELIVERED', 'FAILED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'enum_notification_event') THEN
        CREATE TYPE enum_notification_event AS ENUM (
            'new.transaction',
            'accountant.activity.alert',
            'system.activity.alert'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS tbl_notification (
    id                  UUID                     PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_id        UUID                     NOT NULL,
    recipient_type      VARCHAR(64)              NOT NULL, -- 'PRACTITIONER' | 'ACCOUNTANT' | 'SYSTEM'
    sender_id           UUID                     NULL,
    sender_type         VARCHAR(64)              NULL,     -- 'PRACTITIONER' | 'ACCOUNTANT' | 'SYSTEM'
    event_type          VARCHAR(64)              NOT NULL, -- e.g. 'form.submitted', 'invite.sent'
    entity_type         VARCHAR(64)              NOT NULL, -- e.g. 'form', 'clinic', 'transaction'
    entity_id           UUID                     NOT NULL,
    status              enum_notification_status NOT NULL DEFAULT 'UNREAD',
    read_at             TIMESTAMPTZ              NULL,
    payload             JSONB                    NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ              NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tbl_notification_delivery (
    id                  UUID                 PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id     UUID                 NOT NULL REFERENCES tbl_notification(id) ON DELETE CASCADE,
    channel             VARCHAR(16)          NOT NULL, -- 'in_app' | 'push' | 'email'
    status              enum_delivery_status NOT NULL DEFAULT 'PENDING',
    retry_count         INT                  NOT NULL DEFAULT 0,
    last_attempted_at   TIMESTAMPTZ          NULL,
    delivered_at        TIMESTAMPTZ          NULL,
    error_message       TEXT                 NULL,

    CONSTRAINT uq_notification_channel UNIQUE (notification_id, channel)
);

CREATE TABLE IF NOT EXISTS tbl_notification_preferences (
    id              UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID                    NOT NULL REFERENCES tbl_user(id) ON DELETE CASCADE,
    entity_id       UUID                    NOT NULL,                 -- maps onto individual account role id mappings
    entity_type     VARCHAR(64)             NOT NULL,                 -- 'PRACTITIONER' | 'ACCOUNTANT'
    event_type      enum_notification_event NOT NULL,
    channels        JSONB                   NOT NULL DEFAULT jsonb_build_object(
        'in_app', true,
        'email', true,
        'push', true
    ),
    created_at      TIMESTAMPTZ             NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ             NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ             NULL,

    CONSTRAINT uq_user_entity_event UNIQUE (user_id, entity_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_recipient_unread 
    ON tbl_notification (recipient_id, status) 
    WHERE status = 'UNREAD';

CREATE INDEX IF NOT EXISTS idx_notification_delivery_lookup 
    ON tbl_notification_delivery (notification_id, status);

CREATE INDEX IF NOT EXISTS idx_notif_prefs_active 
    ON tbl_notification_preferences (user_id, event_type) 
    WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TABLE IF EXISTS tbl_notification_delivery;
DROP TABLE IF EXISTS tbl_notification;

DROP TYPE IF EXISTS enum_notification_event;
DROP TYPE IF EXISTS enum_delivery_status;
DROP TYPE IF EXISTS enum_notification_status;
-- +goose StatementEnd