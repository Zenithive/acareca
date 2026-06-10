-- +goose Up
-- +goose StatementBegin

-- 1. CLEAN UP tbl_custom_form_version
-- Ensure practitioner_id exists and is synced, then remove the accountant-specific entity_id
DO $$ 
BEGIN 
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tbl_custom_form_version' AND column_name='entity_id') THEN
        -- If entity_id was being used as the owner, move it to practitioner_id first
        UPDATE tbl_custom_form_version SET practitioner_id = entity_id WHERE practitioner_id IS NULL;
        ALTER TABLE tbl_custom_form_version DROP COLUMN entity_id;
    END IF;
END $$;

-- 2. CLEAN UP tbl_clinic
-- Restore practitioner_id as the sole owner and remove the entity_id link
DO $$ 
BEGIN 
    -- If entity_id exists, rename it back to practitioner_id or drop it if practitioner_id is already present
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tbl_clinic' AND column_name='entity_id') THEN
        -- If practitioner_id doesn't exist, rename entity_id to it
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tbl_clinic' AND column_name='practitioner_id') THEN
            ALTER TABLE tbl_clinic RENAME COLUMN entity_id TO practitioner_id;
        ELSE
            -- If both exist, sync then drop the redundant entity_id
            UPDATE tbl_clinic SET practitioner_id = entity_id WHERE practitioner_id IS NULL;
            ALTER TABLE tbl_clinic DROP COLUMN entity_id;
        END IF;
    END IF;
END $$;

-- 3. Ensure Foreign Keys and Indexes are clean
ALTER TABLE tbl_clinic DROP CONSTRAINT IF EXISTS tbl_clinic_practitioner_id_fkey;
ALTER TABLE tbl_clinic ADD CONSTRAINT tbl_clinic_practitioner_id_fkey 
    FOREIGN KEY (practitioner_id) REFERENCES tbl_practitioner(id);

CREATE INDEX IF NOT EXISTS idx_custom_form_practitioner_id ON tbl_custom_form_version(practitioner_id);
CREATE INDEX IF NOT EXISTS idx_tbl_clinic_practitioner_id ON tbl_clinic(practitioner_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reversing this would mean adding entity_id back, but usually, we don't want to revert to a broken 1-to-1 design.
ALTER TABLE tbl_custom_form_version ADD COLUMN entity_id UUID;
ALTER TABLE tbl_clinic ADD COLUMN entity_id UUID;
-- +goose StatementEnd