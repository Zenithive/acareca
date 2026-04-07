-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_clinic RENAME COLUMN practitioner_id TO entity_id;
ALTER TABLE tbl_clinic DROP CONSTRAINT IF EXISTS tbl_clinic_practitioner_id_fkey;
CREATE INDEX IF NOT EXISTS idx_tbl_clinic_entity_id ON tbl_clinic(entity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tbl_clinic_entity_id;
ALTER TABLE tbl_clinic RENAME COLUMN entity_id TO practitioner_id;
ALTER TABLE tbl_clinic ADD CONSTRAINT tbl_clinic_practitioner_id_fkey 
    FOREIGN KEY (practitioner_id) REFERENCES tbl_practitioner(id);
-- +goose StatementEnd