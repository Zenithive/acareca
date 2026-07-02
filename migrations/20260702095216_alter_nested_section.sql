-- +goose Up
-- +goose StatementBegin

-- Add parent_id column to tbl_invoice_item to support nested entries (children)
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS parent_id UUID,
    ADD CONSTRAINT fk_invoice_item_parent
        FOREIGN KEY (parent_id)
        REFERENCES tbl_invoice_item(id);

-- Add index for parent_id lookups
CREATE INDEX IF NOT EXISTS idx_invoice_item_parent_id
    ON tbl_invoice_item(parent_id)
    WHERE deleted_at IS NULL;

-- Add parent_section_id column to tbl_map_invoice_section to support nested sections
ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS parent_section_id UUID,
    ADD CONSTRAINT fk_invoice_section_parent
        FOREIGN KEY (parent_section_id)
        REFERENCES tbl_map_invoice_section(id);

-- Add index for parent_section_id lookups
CREATE INDEX IF NOT EXISTS idx_invoice_section_parent_id
    ON tbl_map_invoice_section(parent_section_id)
    WHERE deleted_at IS NULL;

-- Add template_id column to tbl_map_invoice_section if it doesn't exist
ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS template_id UUID;

-- Add is_final column to tbl_invoice_item if it doesn't exist
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS is_final BOOLEAN DEFAULT false;



-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Rename bsb back to bsb_number if it exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tbl_map_invoice_section' 
        AND column_name = 'bsb'
    ) THEN
        ALTER TABLE tbl_map_invoice_section RENAME COLUMN bsb TO bsb_number;
    END IF;
END $$;

-- Drop is_final column
ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS is_final;

-- Drop template_id column
ALTER TABLE tbl_map_invoice_section
    DROP COLUMN IF EXISTS template_id;

-- Drop parent_section_id index and column
DROP INDEX IF EXISTS idx_invoice_section_parent_id;
ALTER TABLE tbl_map_invoice_section
    DROP CONSTRAINT IF EXISTS fk_invoice_section_parent,
    DROP COLUMN IF EXISTS parent_section_id;

-- Drop parent_id index and column
DROP INDEX IF EXISTS idx_invoice_item_parent_id;
ALTER TABLE tbl_invoice_item
    DROP CONSTRAINT IF EXISTS fk_invoice_item_parent,
    DROP COLUMN IF EXISTS parent_id;

-- +goose StatementEnd
