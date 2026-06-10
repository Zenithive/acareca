-- +goose Up
CREATE TABLE IF NOT EXISTS tbl_audit_log (
    id              VARCHAR(40) PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    practice_id     VARCHAR(40),        -- NULL for superadmin actions
    user_id         VARCHAR(40),        -- NULL for system/worker actions
    action          TEXT NOT NULL,      -- e.g. entry.confirmed, invitation.sent, bas.lodged
    module          TEXT NOT NULL,      -- identity | clinic | forms | reporting | billing
    entity_type     TEXT,               -- e.g. tbl_custom_form_entry, clinic
    entity_id       VARCHAR(40),        -- ID of the affected row
    before_state    JSONB,              -- Previous values (UPDATE operations)
    after_state     JSONB,              -- New values
    ip_address      VARCHAR(45),        -- IPv4 or IPv6
    user_agent      TEXT,               -- Browser/client info
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Create indexes for common query patterns
CREATE INDEX idx_audit_log_practice_id ON tbl_audit_log(practice_id) WHERE practice_id IS NOT NULL;
CREATE INDEX idx_audit_log_user_id ON tbl_audit_log(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_audit_log_created_at ON tbl_audit_log(created_at DESC);
CREATE INDEX idx_audit_log_module ON tbl_audit_log(module);
CREATE INDEX idx_audit_log_action ON tbl_audit_log(action);
CREATE INDEX idx_audit_log_entity ON tbl_audit_log(entity_type, entity_id) WHERE entity_type IS NOT NULL;

-- Revoke UPDATE and DELETE permissions, allow only INSERT
-- Note: This assumes the app connects as DB_USER from .env
REVOKE UPDATE, DELETE ON tbl_audit_log FROM PUBLIC;
REVOKE UPDATE, DELETE ON tbl_audit_log FROM CURRENT_USER;

-- Grant only INSERT and SELECT permissions
GRANT SELECT, INSERT ON tbl_audit_log TO CURRENT_USER;

-- Add comment for documentation
COMMENT ON TABLE tbl_audit_log IS 'Append-only audit trail for all state-changing operations. UPDATE and DELETE are revoked at database level.';

-- +goose Down
DROP TABLE IF EXISTS tbl_audit_log;
