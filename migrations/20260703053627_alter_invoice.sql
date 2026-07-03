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

CREATE TYPE invoice_method AS ENUM ('SFA_CLINIC_COLLECTS', 'SFA_DENTIST_COLLECTS', 'INDEPENDENT_CONTRACTOR');

ALTER TABLE tbl_invoice 
    ADD COLUMN invoice_method invoice_method NOT NULL DEFAULT 'SFA_CLINIC_COLLECTS';

CREATE TYPE invoice_section_v2 AS ENUM (
    'CALCULATION_STATEMENT',
    'SFA_INVOICE',
    'REMITTANCE_INVOICE',
    'RCTI'
);

ALTER TABLE tbl_map_invoice_section
    DROP COLUMN invoice_section,
    ADD COLUMN invoice_section invoice_section_v2;


CREATE TYPE contact_person_role AS ENUM (
    'DENTIST',
    'PATIENT'
);

ALTER TABLE tbl_clinic_contact_person
    ADD COLUMN IF NOT EXISTS role contact_person_role DEFAULT 'PATIENT';


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin



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

ALTER TABLE tbl_invoice 
    DROP COLUMN invoice_method;

DROP TYPE IF EXISTS invoice_method;
DROP TYPE IF EXISTS invoice_section_v2;

ALTER TABLE tbl_map_invoice_section
    DROP COLUMN invoice_section,
    ADD COLUMN invoice_section invoice_section NOT NULL;

DROP TYPE IF EXISTS contact_person_role;

ALTER TABLE tbl_clinic_contact_person
    DROP COLUMN IF EXISTS role;
-- +goose StatementEnd
