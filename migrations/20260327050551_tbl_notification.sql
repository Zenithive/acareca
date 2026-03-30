-- +goose Up
-- +goose StatementBegin

-- User-facing status: tracks what the recipient has done with the notification.
-- Delivery state per channel is tracked separately in tbl_notification_delivery.
CREATE TYPE enum_notification_status AS ENUM (
    'UNREAD',     -- created, recipient has not opened it yet
    'READ',       -- recipient opened it
    'DISMISSED'   -- recipient archived / cleared it
);

-- Per-channel delivery status: one row per (notification, channel).
CREATE TYPE enum_delivery_status AS ENUM (
    'PENDING',    -- queued, not yet attempted
    'DELIVERED',  -- successfully sent via this channel
    'FAILED'      -- all retries exhausted for this channel
);

CREATE TABLE IF NOT EXISTS tbl_notification (
    id                  UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Who receives the notification
    recipient_id        UUID                        NOT NULL,
    recipient_type      VARCHAR(64)                 NOT NULL,   -- 'PRACTITIONER' | 'ACCOUNTANT' | 'SYSTEM'

    -- Who triggered it (nullable — system-generated notifications have no sender)
    sender_id           UUID,
    sender_type         VARCHAR(64),                            -- 'PRACTITIONER' | 'ACCOUNTANT' | 'SYSTEM'

    -- What happened
    event_type          VARCHAR(64)                 NOT NULL,   -- e.g. 'form.submitted', 'invite.sent'
    entity_type         VARCHAR(64)                 NOT NULL,   -- e.g. 'form', 'clinic', 'transaction'
    entity_id           UUID                        NOT NULL,

    -- User-facing state (not delivery state)
    status              enum_notification_status    NOT NULL DEFAULT 'UNREAD',
    read_at             TIMESTAMPTZ,

    -- Render data — everything the frontend needs without an extra fetch
    payload             JSONB                       NOT NULL DEFAULT '{}',
    -- payload shape:
    -- {
    --   "title":       "Form submitted",
    --   "body":        "Sarah submitted Form #12",
    --   "sender_name": "Sarah Jones",
    --   "entity_name": "Intake Form",
    --   "extra_data":  { "changed_fields": ["title", "status"] }
    -- }

    created_at          TIMESTAMPTZ                 NOT NULL DEFAULT NOW()
);

-- Per-channel delivery tracking.
-- One row is inserted per channel when a notification is published.
CREATE TABLE IF NOT EXISTS tbl_notification_delivery (
    id                  UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id     UUID                        NOT NULL REFERENCES tbl_notification(id) ON DELETE CASCADE,
    channel             VARCHAR(16)                 NOT NULL,   -- 'in_app' | 'push' | 'email'
    status              enum_delivery_status        NOT NULL DEFAULT 'PENDING',
    retry_count         INT                         NOT NULL DEFAULT 0,
    last_attempted_at   TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    error_message       TEXT,

    UNIQUE (notification_id, channel)
);

CREATE TABLE IF NOT EXISTS tbl_notification_preferences (
    entity_id       UUID        NOT NULL,
    entity_type     VARCHAR(64) NOT NULL,   -- 'PRACTITIONER' | 'ACCOUNTANT'
    event_type      VARCHAR(64) NOT NULL,   -- e.g. 'form.submitted'
    channels        JSONB       NOT NULL DEFAULT '["in_app"]', -- ["in_app","push","email"]

    PRIMARY KEY (entity_id, entity_type, event_type)
);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TABLE IF EXISTS tbl_notification_delivery;
DROP TABLE IF EXISTS tbl_notification;
DROP TYPE  IF EXISTS enum_delivery_status;
DROP TYPE  IF EXISTS enum_notification_status;

-- +goose StatementEnd
