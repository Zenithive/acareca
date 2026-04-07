-- +goose Up
-- +goose StatementBegin

CREATE TYPE invite_entity_type AS ENUM (
    'CLINIC',
    'FORM',
    'ENTRY'
);

CREATE TABLE IF NOT EXISTS tbl_invite_permissions (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    practitioner_id       UUID NOT NULL REFERENCES tbl_practitioner(id),
    accountant_id UUID NOT NULL REFERENCES tbl_accountant(id),
    entity_id UUID NOT NULL, 
    entity_type invite_entity_type    NOT NULL, 
    permissions    JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    -- Unique constraint to ensure one permission set per Prac/Acc pair per Entity
    CONSTRAINT unique_permission_scope UNIQUE (practitioner_id, accountant_id, entity_id, entity_type)
);

-- Indexing for performance
CREATE INDEX idx_invite_perms_prac ON tbl_invite_permissions (practitioner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invite_perms_acc ON tbl_invite_permissions (accountant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invite_perms_entity ON tbl_invite_permissions (entity_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_invite_perms_entity_type ON tbl_invite_permissions (entity_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_invite_permissions_lookup ON tbl_invite_permissions (practitioner_id, accountant_id) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tbl_invite_permissions;
DROP TYPE IF EXISTS invite_entity_type;

-- +goose StatementEnd