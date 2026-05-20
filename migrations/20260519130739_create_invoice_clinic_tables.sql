-- +goose Up
-- +goose StatementBegin

-- Create Contact Type Enum 
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invoice_clinic_contact_type') THEN
        CREATE TYPE invoice_clinic_contact_type AS ENUM ('PHONE', 'WEBSITE');
    END IF;
END $$;

DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'verification_token_status') THEN
        CREATE TYPE verification_token_status AS ENUM ('PENDING', 'USED', 'EXPIRED', 'RESENT');
    END IF;
END $$;

-- Clinic Table (Includes Authentication Fields)
CREATE TABLE IF NOT EXISTS tbl_invoice_clinic (
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    document_id VARCHAR(255) NULL,
    clinic_name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password      VARCHAR(100), -- Argon2id; NULL = OAuth-only user
    role          VARCHAR(50),  -- role 
    verified       BOOLEAN NOT NULL DEFAULT FALSE,
    abn VARCHAR(11) NULL,
    acn VARCHAR(9) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

-- Addresses Table
CREATE TABLE IF NOT EXISTS tbl_invoice_clinic_address (
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    address TEXT NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(50) NOT NULL,
    postcode VARCHAR(20) NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

-- Contacts Table
CREATE TABLE IF NOT EXISTS tbl_invoice_clinic_contacts (
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    contact_type invoice_clinic_contact_type NOT NULL,
    value VARCHAR(255) NOT NULL,
    label VARCHAR(100) NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

-- Session Table
CREATE TABLE IF NOT EXISTS tbl_clinic_session (
    id            UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    clinic_id       UUID NOT NULL REFERENCES tbl_invoice_clinic(id),
    refresh_token TEXT         NOT NULL,
    user_agent    TEXT,
    ip_address    VARCHAR(45),
    expires_at    TIMESTAMPTZ  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ  NULL
);

-- Verification Token Table
CREATE TABLE tbl_clinic_verification_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL, 
    role VARCHAR(50) NOT NULL,      
    status verification_token_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);


-- Index optimization for structural lookup operations
CREATE INDEX IF NOT EXISTS idx_inv_clinic_email ON tbl_invoice_clinic(email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_addr_lookup ON tbl_invoice_clinic_address(clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_cont_lookup ON tbl_invoice_clinic_contacts(clinic_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inv_clinic_verification_token_entity ON tbl_clinic_verification_token(clinic_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_clinic_session;
DROP TABLE IF EXISTS tbl_clinic_verification_token;
DROP TABLE IF EXISTS tbl_invoice_clinic_contacts;
DROP TABLE IF EXISTS tbl_invoice_clinic_address;
DROP TABLE IF EXISTS tbl_invoice_clinic;
DROP TYPE IF EXISTS invoice_clinic_contact_type;
-- +goose StatementEnd