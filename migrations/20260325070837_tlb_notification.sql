-- +goose Up
-- +goose StatementBegin

CREATE TYPE enum_notification_status AS ENUM (
    'PENDING',
    'DELIVERED',
    'READ',
    'DISMISSED',
    'FAILED'
);

-- Main tbl_notification table
CREATE TABLE IF NOT EXISTS tbl_notification (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipient_id    UUID        NOT NULL,
    sender_id       UUID        NULL,
    event_type      VARCHAR(64) NOT NULL,
    entity_type     VARCHAR(64) NOT NULL,
    entity_id       UUID        NOT NULL,
    status          enum_notification_status NOT NULL DEFAULT 'PENDING',
    payload         JSONB       NOT NULL DEFAULT '{}',
    retry_count     INT         NOT NULL DEFAULT 0,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_recipient_status
    ON tbl_notification (recipient_id, status);

CREATE INDEX idx_notifications_recipient_created
    ON tbl_notification (recipient_id, created_at DESC);

-- Outbox table (written in same tx as the triggering action)
CREATE TABLE IF NOT EXISTS tbl_notification_outbox (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      VARCHAR(64)  NOT NULL,
    actor_id        UUID         NOT NULL,
    entity_type     VARCHAR(64)  NOT NULL,
    entity_id       UUID         NOT NULL,
    payload         JSONB        NOT NULL DEFAULT '{}',
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_unprocessed
    ON tbl_notification_outbox (created_at ASC)
    WHERE processed_at IS NULL;

-- Per-user, per-event-type channel preferences
CREATE TABLE IF NOT EXISTS tbl_notification_preferences (
    user_id     UUID        NOT NULL,
    event_type  VARCHAR(64) NOT NULL,
    -- JSON array of channels: ["in_app","email","push"]
    channels    JSONB       NOT NULL DEFAULT '["in_app"]',
    PRIMARY KEY (user_id, event_type)
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TABLE IF EXISTS tbl_notification_outbox;
DROP TABLE IF EXISTS tbl_notification;
DROP TYPE IF EXISTS enum_notification_status;

-- +goose StatementEnd