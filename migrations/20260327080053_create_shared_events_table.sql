-- +goose Up
-- +goose StatementBegin

-- Shared Events: Acts as a universal bridge/ledger between Accountants and Practitioners.
-- This table tracks every collaborative action (CRUD) performed on shared entities.

-- Defines the type of actor performing the action
CREATE TYPE enum_shared_event_actor AS ENUM (
    'ACCOUNTANT',
    'PRACTITIONER'
);

-- Defines the category of entity being touched
CREATE TYPE enum_shared_event_entity AS ENUM (
    'CLINIC',
    'FORM',
    'INVITATION'
    
);

CREATE TABLE IF NOT EXISTS tbl_shared_events (
    id              UUID                        PRIMARY KEY DEFAULT gen_random_uuid(),

   
    practitioner_id UUID                        NOT NULL, 
    accountant_id   UUID                        NOT NULL, 
    

    -- Actor Details (Who performed the action)
    
    actor_id        UUID                        NOT NULL, 
    actor_name      VARCHAR(255),                         
    actor_type      enum_shared_event_actor     NOT NULL, 
    
   
    event_type      VARCHAR(64)                 NOT NULL, 
    entity_type     enum_shared_event_entity    NOT NULL, 
    entity_id       UUID                        NOT NULL,-- ID of the actual record touched
    
   
    description     TEXT                        NOT NULL, 
    
    -- Extensible metadata for the activity feed
    metadata        JSONB                       NOT NULL DEFAULT '{}',
  

    created_at      TIMESTAMPTZ                 NOT NULL DEFAULT NOW()
);

-- Indexing for high-performance dashboard activity feeds
CREATE INDEX IF NOT EXISTS idx_shared_events_practitioner ON tbl_shared_events(practitioner_id);
CREATE INDEX IF NOT EXISTS idx_shared_events_accountant   ON tbl_shared_events(accountant_id);
CREATE INDEX IF NOT EXISTS idx_shared_events_created_at   ON tbl_shared_events(created_at DESC);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_shared_events_created_at;
DROP INDEX IF EXISTS idx_shared_events_accountant;
DROP INDEX IF EXISTS idx_shared_events_practitioner;

DROP TABLE IF EXISTS tbl_shared_events;

-- +goose StatementEnd