-- +goose Up
-- +goose StatementBegin

DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invitation_status') THEN
        CREATE TYPE invitation_status AS ENUM ('SENT', 'ACCEPTED', 'COMPLETED', 'REJECTED', 'REVOKED');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS tbl_invitation (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id) ON DELETE CASCADE,
    accountant_id   UUID NULL REFERENCES tbl_accountant(id) ON DELETE SET NULL, 
    email           VARCHAR(255) NOT NULL,
    status          invitation_status NOT NULL DEFAULT 'SENT',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NULL DEFAULT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    deleted_at      TIMESTAMPTZ NULL DEFAULT NULL    
);

CREATE TABLE IF NOT EXISTS tbl_permission (
    id   SMALLINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(50) NOT NULL UNIQUE
);

INSERT INTO tbl_permission (name) VALUES 
    ('sales_purchases'),
    ('lock_dates'),
    ('manage_users'),
    ('reports_view_download')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS tbl_invite_permissions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invitation_id UUID NOT NULL REFERENCES tbl_invitation(id) ON DELETE CASCADE,
    permission_id SMALLINT NOT NULL REFERENCES tbl_permission(id) ON DELETE RESTRICT,
    can_read      BOOLEAN NOT NULL DEFAULT false,
    can_write     BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NULL DEFAULT NULL,
    deleted_at    TIMESTAMPTZ NULL DEFAULT NULL,
    
    CONSTRAINT uq_permission_invitation UNIQUE (invitation_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_invitation_practitioner ON tbl_invitation (practitioner_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitation_accountant ON tbl_invitation (accountant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invitation_email ON tbl_invitation (email) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invite_perms_invitation ON tbl_invite_permissions (invitation_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_invite_perms_permission ON tbl_invite_permissions (permission_id) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invite_permissions CASCADE;
DROP TABLE IF EXISTS tbl_permission CASCADE;
DROP TABLE IF EXISTS tbl_invitation CASCADE;

DROP TYPE IF EXISTS invitation_status;
-- +goose StatementEnd