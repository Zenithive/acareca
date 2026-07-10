-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'paid_by') THEN
        CREATE TYPE paid_by AS ENUM (
            'Dentist',
            'Clinic'
        );
    END IF;
END $$;

-- Add parent_id column to tbl_invoice_item to support nested entries (children)
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS parent_id UUID,
    ADD COLUMN IF NOT EXISTS paid_by paid_by NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_invoice_item_parent'
    ) THEN
        ALTER TABLE tbl_invoice_item
            ADD CONSTRAINT fk_invoice_item_parent
                FOREIGN KEY (parent_id)
                REFERENCES tbl_invoice_item(id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invoice_item_parent_id
    ON tbl_invoice_item(parent_id)
    WHERE deleted_at IS NULL;

-- Add parent_section_id column to tbl_map_invoice_section to support nested sections
ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS parent_section_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_invoice_section_parent'
    ) THEN
        ALTER TABLE tbl_map_invoice_section
            ADD CONSTRAINT fk_invoice_section_parent
                FOREIGN KEY (parent_section_id)
                REFERENCES tbl_map_invoice_section(id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_invoice_section_parent_id
    ON tbl_map_invoice_section(parent_section_id)
    WHERE deleted_at IS NULL;

-- Add template_id column to tbl_map_invoice_section if it doesn't exist
ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS template_id UUID;

-- Add is_final column to tbl_invoice_item if it doesn't exist
ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS is_final BOOLEAN DEFAULT false;

-- invoice_method enum + column
DO $$
BEGIN
    CREATE TYPE invoice_method AS ENUM ('SFA_CLINIC_COLLECTS', 'SFA_DENTIST_COLLECTS', 'INDEPENDENT_CONTRACTOR');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS invoice_method invoice_method NOT NULL DEFAULT 'SFA_CLINIC_COLLECTS';

-- invoice_section_v2 enum + column swap (preserves data via text cast)
DO $$
BEGIN
    CREATE TYPE invoice_section_v2 AS ENUM (
        'CALCULATION_STATEMENT',
        'SFA_INVOICE',
        'REMITTANCE_INVOICE',
        'RCTI',
        'PATIENT_FEE',
        'TREATMENT_COST', 
        'NET_PATIENT_FEES', 
        'COMMISSION', 
        'NET_SETTLEMENT', 
        'SERVICE_FACILITY'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tbl_map_invoice_section'
        AND column_name = 'invoice_section'
        AND udt_name <> 'invoice_section_v2'
    ) THEN
        ALTER TABLE tbl_map_invoice_section
            ADD COLUMN invoice_section_new invoice_section_v2;

        -- NOTE: old enum values must exist in invoice_section_v2 or this cast fails.
        -- Adjust the CASE mapping below if old values don't line up 1:1.
        UPDATE tbl_map_invoice_section
        SET invoice_section_new = invoice_section::text::invoice_section_v2;

        ALTER TABLE tbl_map_invoice_section DROP COLUMN invoice_section;
        ALTER TABLE tbl_map_invoice_section RENAME COLUMN invoice_section_new TO invoice_section;
    END IF;
END $$;

-- contact_person_role enum + column
DO $$
BEGIN
    CREATE TYPE contact_person_role AS ENUM (
        'DENTIST',
        'PATIENT'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

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

ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS is_final;

ALTER TABLE tbl_map_invoice_section
    DROP COLUMN IF EXISTS template_id;

DROP INDEX IF EXISTS idx_invoice_section_parent_id;
ALTER TABLE tbl_map_invoice_section
    DROP CONSTRAINT IF EXISTS fk_invoice_section_parent,
    DROP COLUMN IF EXISTS parent_section_id;

DROP INDEX IF EXISTS idx_invoice_item_parent_id;
ALTER TABLE tbl_invoice_item
    DROP CONSTRAINT IF EXISTS fk_invoice_item_parent,
    DROP COLUMN IF EXISTS parent_id,
    DROP COLUMN IF EXISTS paid_by;

ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS invoice_method;

DROP TYPE IF EXISTS invoice_method;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tbl_map_invoice_section'
        AND column_name = 'invoice_section'
    ) THEN
        ALTER TABLE tbl_map_invoice_section
            ADD COLUMN invoice_section_old invoice_section;
        UPDATE tbl_map_invoice_section
        SET invoice_section_old = invoice_section::text::invoice_section;
        ALTER TABLE tbl_map_invoice_section DROP COLUMN invoice_section;
        ALTER TABLE tbl_map_invoice_section RENAME COLUMN invoice_section_old TO invoice_section;
        ALTER TABLE tbl_map_invoice_section ALTER COLUMN invoice_section SET NOT NULL;
    END IF;
END $$;

DROP TYPE IF EXISTS invoice_section_v2;

ALTER TABLE tbl_clinic_contact_person
    DROP COLUMN IF EXISTS role;

DROP TYPE IF EXISTS contact_person_role;

-- +goose StatementEnd