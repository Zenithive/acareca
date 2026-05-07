-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_document (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    owner_id UUID NOT NULL, -- ownership
    owner_role VARCHAR(20) NOT NULL,

    -- file identity
    object_key TEXT NOT NULL UNIQUE,   -- R2 path
    bucket TEXT NOT NULL,

    -- file metadata
    original_name TEXT NOT NULL,
    extension VARCHAR(20),
    mime_type VARCHAR(100) NOT NULL,

    size_bytes BIGINT NOT NULL,

    -- integrity
    checksum VARCHAR(128), -- sha256 or md5

    -- lifecycle
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- pending | uploaded | failed | deleted

    is_public BOOLEAN NOT NULL DEFAULT FALSE,


    -- optional linking (generic usage)
    entity_type VARCHAR(50),
    entity_id UUID,

    -- upload tracking
    upload_expires_at TIMESTAMP,
    uploaded_at TIMESTAMP,

    -- audit
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_document;
-- +goose StatementEnd
