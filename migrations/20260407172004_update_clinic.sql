-- +goose Up
-- +goose StatementBegin

-- 1. Rename the current 'entity_id' back to 'practitioner_id' to restore the primary owner data
ALTER TABLE tbl_clinic RENAME COLUMN entity_id TO practitioner_id;

-- 2. Restore the foreign key constraint for the practitioner
ALTER TABLE tbl_clinic ADD CONSTRAINT tbl_clinic_practitioner_id_fkey 
    FOREIGN KEY (practitioner_id) REFERENCES tbl_practitioner(id);

-- 3. Add 'entity_id' as a new, separate column
ALTER TABLE tbl_clinic ADD COLUMN IF NOT EXISTS entity_id UUID;

-- 4. Sync data so entity_id matches practitioner_id for now
UPDATE tbl_clinic SET entity_id = practitioner_id WHERE entity_id IS NULL;

-- 5. Add index for the new entity_id
CREATE INDEX IF NOT EXISTS idx_tbl_clinic_entity_id ON tbl_clinic(entity_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tbl_clinic_entity_id;
ALTER TABLE tbl_clinic DROP COLUMN IF EXISTS entity_id;
-- Note: We leave practitioner_id as is to avoid breaking existing data flow
-- +goose StatementEnd