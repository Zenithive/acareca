-- +goose Up
-- +goose StatementBegin

-- 1. Modify columns
ALTER TABLE tbl_invite_permissions ADD COLUMN IF NOT EXISTS email VARCHAR(255);
ALTER TABLE tbl_invite_permissions ALTER COLUMN accountant_id DROP NOT NULL;

-- 2. Drop the old constraint
ALTER TABLE tbl_invite_permissions DROP CONSTRAINT IF EXISTS unique_permission_scope;

-- 3. Index for Pending Invites (when accountant_id is NULL)
-- Ensures: 1 Practitioner + 1 Email + 1 Entity is unique
CREATE UNIQUE INDEX IF NOT EXISTS unique_pending_permission_idx 
ON tbl_invite_permissions (practitioner_id, email, entity_id, entity_type) 
WHERE accountant_id IS NULL AND deleted_at IS NULL;

-- 4. Index for Active Permissions (when accountant_id is NOT NULL)
-- Ensures: 1 Practitioner + 1 Accountant ID + 1 Entity is unique
CREATE UNIQUE INDEX IF NOT EXISTS unique_active_permission_idx 
ON tbl_invite_permissions (practitioner_id, accountant_id, entity_id, entity_type) 
WHERE accountant_id IS NOT NULL AND deleted_at IS NULL;

-- 5. Helper index for registration lookup
CREATE INDEX IF NOT EXISTS idx_invite_perms_email ON tbl_invite_permissions (email) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS unique_pending_permission_idx;
DROP INDEX IF EXISTS unique_active_permission_idx;
DROP INDEX IF EXISTS idx_invite_perms_email;

ALTER TABLE tbl_invite_permissions ADD CONSTRAINT unique_permission_scope 
UNIQUE (practitioner_id, accountant_id, entity_id, entity_type);

ALTER TABLE tbl_invite_permissions ALTER COLUMN accountant_id SET NOT NULL;
ALTER TABLE tbl_invite_permissions DROP COLUMN IF EXISTS email;
-- +goose StatementEnd