-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invite_permissions;
DROP TYPE IF EXISTS invite_entity_type;

CREATE TABLE tbl_invite_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id),
    accountant_id UUID REFERENCES tbl_accountant(id),
    email TEXT,
    permission_name TEXT NOT NULL,
    can_read BOOLEAN NOT NULL DEFAULT false,
    can_write BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
       CONSTRAINT check_accountant_or_email CHECK (
        (accountant_id IS NOT NULL AND email IS NULL) OR 
        (accountant_id IS NULL AND email IS NOT NULL)
    )

);

CREATE UNIQUE INDEX unique_perm_registered 
    ON tbl_invite_permissions (practitioner_id, accountant_id, permission_name) 
    WHERE accountant_id IS NOT NULL;

CREATE UNIQUE INDEX unique_perm_pending 
    ON tbl_invite_permissions (practitioner_id, email, permission_name) 
    WHERE accountant_id IS NULL;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invite_permissions;
-- +goose StatementEnd