-- Add image_url column to tbl_clinic table
ALTER TABLE tbl_clinic ADD COLUMN IF NOT EXISTS image_url TEXT;

-- Add comment to the column
COMMENT ON COLUMN tbl_clinic.image_url IS 'URL of the clinic image';
