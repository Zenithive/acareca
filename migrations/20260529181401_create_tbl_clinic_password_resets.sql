-- +goose Up
-- +goose StatementBegin

-- Create password reset status enum defensively
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'token_status') THEN
        CREATE TYPE token_status AS ENUM ('PENDING', 'USED', 'EXPIRED');
    END IF;
END $$;

-- Create password resets table
CREATE TABLE IF NOT EXISTS tbl_clinic_password_resets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id  UUID NOT NULL REFERENCES tbl_invoice_clinic(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    status     token_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Optimize lookup operation for the token matching query
CREATE INDEX IF NOT EXISTS idx_clinic_password_resets_token 
ON tbl_clinic_password_resets(token_hash) 
WHERE status = 'PENDING';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_clinic_password_resets;
-- Note: password_reset_status is preserved since other module rely on it
-- +goose StatementEnd