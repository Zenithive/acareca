-- +goose Up
-- +goose StatementBegin

-- Add metadata column to tbl_template for storing field schema and computed fields
ALTER TABLE tbl_template
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;

-- Add custom_fields column to tbl_invoice for storing user-entered custom field values
ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS custom_fields JSONB DEFAULT '{}'::jsonb;

-- GIN indexes for efficient JSONB querying
CREATE INDEX IF NOT EXISTS idx_template_metadata 
    ON tbl_template USING GIN (metadata)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_custom_fields
    ON tbl_invoice USING GIN (custom_fields)
    WHERE deleted_at IS NULL;

COMMENT ON COLUMN tbl_template.metadata IS 'Stores field schema definitions and computed field formulas as JSONB';
COMMENT ON COLUMN tbl_invoice.custom_fields IS 'Stores user-entered custom field values as JSONB';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_invoice_custom_fields;
DROP INDEX IF EXISTS idx_template_metadata;

ALTER TABLE tbl_invoice DROP COLUMN IF EXISTS custom_fields;
ALTER TABLE tbl_template DROP COLUMN IF EXISTS metadata;

-- +goose StatementEnd
