-- +goose Up
-- +goose StatementBegin

-- First, rename entity_id to accountant_id in tbl_invitation for clarity
ALTER TABLE tbl_invitation RENAME COLUMN entity_id TO accountant_id;

-- Drop the old tbl_invite_permissions table if it exists
DROP TABLE IF EXISTS tbl_invite_permissions CASCADE;
DROP TYPE IF EXISTS invite_entity_type;

-- Create the permission reference table
CREATE TABLE IF NOT EXISTS tbl_permission (
    id SMALLINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(50) NOT NULL,
    CONSTRAINT uq_permission_name UNIQUE (name)
);

-- Insert the standard permissions
INSERT INTO tbl_permission (name) VALUES 
    ('sales_purchases'),
    ('lock_dates'),
    ('manage_users'),
    ('reports_view_download')
ON CONFLICT (name) DO NOTHING;

-- Create the new invite_permissions table linking invitations to permissions
CREATE TABLE IF NOT EXISTS tbl_invite_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invitation_id UUID NOT NULL REFERENCES tbl_invitation(id) ON DELETE CASCADE,
    permission_id SMALLINT NOT NULL REFERENCES tbl_permission(id) ON DELETE RESTRICT,
    can_read BOOLEAN NOT NULL DEFAULT false,
    can_write BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_permission_invitation UNIQUE (invitation_id, permission_id)
);

-- Create indexes for performance
CREATE INDEX idx_invite_perms_invitation ON tbl_invite_permissions (invitation_id);
CREATE INDEX idx_invite_perms_permission ON tbl_invite_permissions (permission_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop the new tables
DROP TABLE IF EXISTS tbl_invite_permissions CASCADE;
DROP TABLE IF EXISTS tbl_permission CASCADE;

-- Rename accountant_id back to entity_id in tbl_invitation
ALTER TABLE tbl_invitation RENAME COLUMN accountant_id TO entity_id;

-- +goose StatementEnd
