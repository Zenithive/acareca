-- +goose Up

-- 1. Add the column (Allow NULL for now so the update can happen)
ALTER TABLE tbl_financial_settings 
ADD COLUMN IF NOT EXISTS practitioner_id UUID REFERENCES tbl_practitioner(id);

-- 2. Run the Mapping (The query from above)
UPDATE tbl_financial_settings fs
SET practitioner_id = c.practitioner_id
FROM tbl_clinic c
WHERE fs.clinic_id = c.id;

-- 3. Cleanup: Remove any settings that couldn't be mapped to a practitioner
-- (e.g., if a clinic was deleted but the setting remained)
DELETE FROM tbl_financial_settings WHERE practitioner_id IS NULL;

-- 4. Enforce NOT NULL now that data is populated
ALTER TABLE tbl_financial_settings ALTER COLUMN practitioner_id SET NOT NULL;

-- 5. Optional: Make clinic_id nullable so it doesn't cause issues
ALTER TABLE tbl_financial_settings ALTER COLUMN clinic_id DROP NOT NULL;


-- +goose Down
ALTER TABLE tbl_financial_settings ALTER COLUMN clinic_id SET NOT NULL;
ALTER TABLE tbl_financial_settings DROP COLUMN IF EXISTS practitioner_id;