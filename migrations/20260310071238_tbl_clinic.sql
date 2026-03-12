-- +goose Up

CREATE TABLE IF NOT EXISTS tbl_clinic (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(), 
    practitioner_id    UUID NOT NULL REFERENCES tbl_practitioner(id),
    profile_picture TEXT, 
    name            TEXT NOT NULL, 
    abn             VARCHAR(11), 
    description     TEXT, 
    is_active       BOOLEAN NOT NULL DEFAULT TRUE, 
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(), 
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ                       -- Soft delete
);

-- +goose Down
DROP TABLE IF EXISTS tbl_clinic;