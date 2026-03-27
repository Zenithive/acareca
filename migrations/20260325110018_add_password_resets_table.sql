-- +goose Up
-- +goose StatementBegin

-- 1. Create the ENUM type first if it doesn't exist
DO $$ BEGIN
    CREATE TYPE token_status AS ENUM ('PENDING', 'USED', 'EXPIRED');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS tbl_password_resets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES tbl_user(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL, 
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_resets_user ON tbl_password_resets(user_id);
CREATE INDEX IF NOT EXISTS idx_password_resets_token ON tbl_password_resets(token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tbl_password_resets;
-- +goose StatementEnd