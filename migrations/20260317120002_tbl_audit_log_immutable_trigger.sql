-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fn_audit_log_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'tbl_audit_log is append-only: % operations are not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_audit_log_no_update
    BEFORE UPDATE ON tbl_audit_log
    FOR EACH ROW EXECUTE FUNCTION fn_audit_log_immutable();

CREATE TRIGGER trg_audit_log_no_delete
    BEFORE DELETE ON tbl_audit_log
    FOR EACH ROW EXECUTE FUNCTION fn_audit_log_immutable();

-- +goose Down
DROP TRIGGER IF EXISTS trg_audit_log_no_update ON tbl_audit_log;
DROP TRIGGER IF EXISTS trg_audit_log_no_delete ON tbl_audit_log;
-- +goose StatementBegin
DROP FUNCTION IF EXISTS fn_audit_log_immutable();
-- +goose StatementEnd
