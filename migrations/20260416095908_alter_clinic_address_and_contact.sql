-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_clinic_address ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE tbl_clinic_contact ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;


-- Indices for performance
CREATE INDEX idx_clinic_address_deleted_at ON tbl_clinic_address (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_clinic_contact_deleted_at ON tbl_clinic_contact (deleted_at) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_clinic_address_deleted_at;
DROP INDEX IF EXISTS idx_clinic_contact_deleted_at;

ALTER TABLE tbl_clinic_address DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE tbl_clinic_contact DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd