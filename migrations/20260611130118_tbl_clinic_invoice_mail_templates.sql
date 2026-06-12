-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_clinic_invoice_mail_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL UNIQUE,
    mail_subject TEXT NOT NULL,
    mail_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL 
);

-- Index optimization to query configurations quickly via clinic_id contexts
CREATE INDEX IF NOT EXISTS idx_invoice_mail_templates_clinic_id 
ON tbl_clinic_invoice_mail_templates(clinic_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_clinic_invoice_mail_templates;
-- +goose StatementEnd