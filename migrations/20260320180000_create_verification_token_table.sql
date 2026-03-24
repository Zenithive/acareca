-- +goose Up
-- +goose StatementBegin
CREATE TYPE verification_token_status AS ENUM ('PENDING', 'USED', 'EXPIRED', 'RESENT');

CREATE TABLE tbl_verification_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL, 
    role VARCHAR(50),        -- For now Nullable
    status verification_token_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_verification_token_entity ON tbl_verification_token(entity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tbl_verification_token;
DROP TYPE verification_token_status;
-- +goose StatementEnd