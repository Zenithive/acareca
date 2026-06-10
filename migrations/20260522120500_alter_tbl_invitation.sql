-- +goose Up
-- +goose StatementBegin
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invitation_status') THEN
        CREATE TYPE invitation_status AS ENUM ('SENT', 'ACCEPTED', 'COMPLETED', 'REJECTED', 'REVOKED');
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE tbl_invitation 
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE tbl_invite_permissions 
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS deleted_at,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ DEFAULT NULL;

-- +goose Down
ALTER TABLE tbl_invitation 
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE tbl_invite_permissions
    DROP COLUMN IF EXISTS updated_at,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
