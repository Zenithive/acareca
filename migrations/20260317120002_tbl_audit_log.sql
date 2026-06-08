-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tbl_audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id     UUID NULL,          
    user_id         UUID NULL,          
    action          TEXT NOT NULL,      
    module          TEXT NOT NULL,      
    entity_type     TEXT NULL,          
    entity_id       UUID NULL,          
    before_state    JSONB NULL,         
    after_state     JSONB NULL,         
    ip_address      VARCHAR(45) NULL,   
    user_agent      TEXT NULL,          
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_practice_id ON tbl_audit_log(practice_id) WHERE practice_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON tbl_audit_log(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON tbl_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_module ON tbl_audit_log(module);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON tbl_audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON tbl_audit_log(entity_type, entity_id) WHERE entity_type IS NOT NULL;

CREATE OR REPLACE FUNCTION fn_audit_log_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'tbl_audit_log is append-only: % operations are strictly prohibited.', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_log_no_update
    BEFORE UPDATE ON tbl_audit_log
    FOR EACH ROW EXECUTE FUNCTION fn_audit_log_immutable();

CREATE TRIGGER trg_audit_log_no_delete
    BEFORE DELETE ON tbl_audit_log
    FOR EACH ROW EXECUTE FUNCTION fn_audit_log_immutable();

REVOKE UPDATE, DELETE ON tbl_audit_log FROM PUBLIC;
REVOKE UPDATE, DELETE ON tbl_audit_log FROM CURRENT_USER;
GRANT SELECT, INSERT ON tbl_audit_log TO CURRENT_USER;

COMMENT ON TABLE tbl_audit_log IS 'Append-only audit trail tracking state-changing platform operations. UPDATE and DELETE actions are hard-blocked at database and procedural engine levels.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_audit_log_no_delete ON tbl_audit_log;
DROP TRIGGER IF EXISTS trg_audit_log_no_update ON tbl_audit_log;
DROP FUNCTION IF EXISTS fn_audit_log_immutable();

DROP TABLE IF EXISTS tbl_audit_log;
-- +goose StatementEnd