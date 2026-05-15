-- +goose Up
-- +goose StatementBegin

-- Create the Business Entity Enum
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'business_entity_type') THEN
        CREATE TYPE business_entity_type AS ENUM ('SOLE_TRADER', 'COMPANY', 'TRUST');
    END IF;
END $$;

-- Update tbl_practitioner
ALTER TABLE tbl_practitioner 
    ADD COLUMN IF NOT EXISTS entity_type business_entity_type NOT NULL DEFAULT 'SOLE_TRADER',
    ADD COLUMN IF NOT EXISTS entity_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS address TEXT,
    ADD COLUMN IF NOT EXISTS acn VARCHAR(9),
    ADD COLUMN IF NOT EXISTS profession VARCHAR(100);

-- Update tbl_accountant
-- Rename license_no to tax_agent_number (TAN) if it exists, otherwise create it
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tbl_accountant' AND column_name='license_no') THEN
        ALTER TABLE tbl_accountant RENAME COLUMN license_no TO tax_agent_number;
    ELSIF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tbl_accountant' AND column_name='tax_agent_number') THEN
        ALTER TABLE tbl_accountant ADD COLUMN tax_agent_number VARCHAR(50);
    END IF;
END $$;

ALTER TABLE tbl_accountant 
    ADD COLUMN IF NOT EXISTS abn VARCHAR(20),
    ADD COLUMN IF NOT EXISTS entity_type business_entity_type NOT NULL DEFAULT 'SOLE_TRADER',
    ADD COLUMN IF NOT EXISTS entity_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS address TEXT,
    ADD COLUMN IF NOT EXISTS acn VARCHAR(9),
    ADD COLUMN IF NOT EXISTS profession VARCHAR(100);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_practitioner 
    DROP COLUMN IF EXISTS entity_type,
    DROP COLUMN IF EXISTS entity_name,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS acn,
    DROP COLUMN IF EXISTS profession;

-- Revert TAN back to license_no if you want to keep original state
ALTER TABLE tbl_accountant RENAME COLUMN tax_agent_number TO license_no;

ALTER TABLE tbl_accountant 
    DROP COLUMN IF EXISTS abn,
    DROP COLUMN IF EXISTS entity_type,
    DROP COLUMN IF EXISTS entity_name,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS acn,
    DROP COLUMN IF EXISTS profession;

DROP TYPE IF EXISTS business_entity_type;

-- +goose StatementEnd