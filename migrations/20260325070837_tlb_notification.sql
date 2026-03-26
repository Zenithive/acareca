-- +goose Up
-- +goose StatementBegin

CREATE TYPE enum_notification_status AS ENUM (
    'PENDING',    -- created, not yet delivered
    'DELIVERED',  -- pushed / emailed to recipient
    'READ',       -- recipient opened it
    'DISMISSED',  -- recipient archived it
    'FAILED'      -- all retries exhausted
);

CREATE TABLE IF NOT EXISTS tbl_notification (
    id                  UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Who receives the notification
    recipient_id        UUID                        NOT NULL,
    recipient_type      VARCHAR(64)                 NOT NULL,   -- 'practitioner' | 'account' | etc.

    -- Who triggered it (nullable — system-generated notifications have no sender)
    sender_id           UUID,
    sender_type         VARCHAR(64),                            -- 'practitioner' | 'account' | 'system'

    -- What happened
    event_type          VARCHAR(64)                 NOT NULL,   -- e.g. 'form.submitted', 'invite.sent'
    entity_type         VARCHAR(64)                 NOT NULL,   -- e.g. 'form', 'clinic', 'transaction'
    entity_id           UUID                        NOT NULL,

    -- State
    status              enum_notification_status    NOT NULL DEFAULT 'PENDING',
    retry_count         INT                         NOT NULL DEFAULT 0,
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


CREATE TABLE IF NOT EXISTS tbl_notification_preferences (
    entity_id       UUID        NOT NULL,
    entity_type     VARCHAR(64) NOT NULL,   -- 'practitioner' | 'account'
    event_type      VARCHAR(64) NOT NULL,   -- e.g. 'form.submitted'
    channels        JSONB       NOT NULL DEFAULT '["in_app"]', --  ["in_app","push","email"]

    PRIMARY KEY (entity_id, entity_type, event_type)
);


-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_notification_preferences;
DROP TABLE IF EXISTS tbl_notification;
DROP TYPE  IF EXISTS enum_notification_status;

-- +goose StatementEnd