-- +goose Up
-- +goose StatementBegin

-- Add template_id column to tbl_map_invoice_section
ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS template_id UUID;

-- Add foreign key constraint to reference tbl_template
ALTER TABLE tbl_map_invoice_section
    ADD CONSTRAINT fk_invoice_section_template
    FOREIGN KEY (template_id)
    REFERENCES tbl_template(id);

-- Create index for better query performance on template_id
CREATE INDEX IF NOT EXISTS idx_invoice_section_template_id
    ON tbl_map_invoice_section(template_id)
    WHERE deleted_at IS NULL;

-- Add comment to document the column purpose
COMMENT ON COLUMN tbl_map_invoice_section.template_id IS 'References the template used for rendering this specific invoice section';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Drop index
DROP INDEX IF EXISTS idx_invoice_section_template_id;

-- Drop foreign key constraint
ALTER TABLE tbl_map_invoice_section
    DROP CONSTRAINT IF EXISTS fk_invoice_section_template;

-- Remove template_id column
ALTER TABLE tbl_map_invoice_section
    DROP COLUMN IF EXISTS template_id;

-- +goose StatementEnd
