-- +goose Up
-- +goose StatementBegin

ALTER TABLE tbl_map_invoice_section DROP CONSTRAINT IF EXISTS fk_invoice_section_template;
ALTER TABLE tbl_map_invoice_section DROP COLUMN IF EXISTS template_id;
DROP INDEX IF EXISTS idx_invoice_section_template_id;

DROP TABLE IF EXISTS tbl_invoice_template_mapping;
DROP TABLE IF EXISTS tbl_template_setting;
DROP TABLE IF EXISTS tbl_template;

CREATE TABLE IF NOT EXISTS tbl_template (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    description TEXT         NULL,
    html         BYTEA        NOT NULL,
    css          BYTEA        NOT NULL,
    is_default   BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NULL,
    deleted_at   TIMESTAMPTZ  NULL
);

CREATE TABLE IF NOT EXISTS tbl_template_setting (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id         UUID NULL,
    primary_color VARCHAR(20) NOT NULL DEFAULT '#1f4e5f',
    accent_color VARCHAR(20) NOT NULL DEFAULT '#5f96b4',
    body_font_family VARCHAR(100) NOT NULL DEFAULT 'Arial',
    header_font_family VARCHAR(100) NOT NULL DEFAULT 'Arial',
    is_logo BOOLEAN NOT NULL DEFAULT FALSE,
    logo_id UUID NULL REFERENCES tbl_document(id),
    terms_text TEXT NULL,
    payment_terms TEXT NULL,
    is_watermark BOOLEAN NOT NULL DEFAULT FALSE,
    watermark_text VARCHAR(100) NULL,
    table_style VARCHAR(100) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

ALTER TABLE tbl_invoice ADD COLUMN invoice_method VARCHAR(1) NOT NULL DEFAULT 'A';

ALTER TABLE tbl_clinic_contact_person ADD COLUMN role VARCHAR(255) DEFAULT NULL;

-- Guarantees only ONE global system fallback row can exist across the platform
CREATE UNIQUE INDEX IF NOT EXISTS idx_tpl_setting_global_default
    ON tbl_template_setting((invoice_id IS NULL))
    WHERE invoice_id IS NULL AND deleted_at IS NULL;

-- Guarantees an invoice cannot accidentally have duplicate specialized layout rows
CREATE UNIQUE INDEX IF NOT EXISTS idx_tpl_setting_invoice_unique
    ON tbl_template_setting(invoice_id)
    WHERE invoice_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE tbl_invoice_item
    ADD COLUMN IF NOT EXISTS is_final BOOLEAN NOT NULL DEFAULT false;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_invoice DROP COLUMN invoice_method;

ALTER TABLE tbl_clinic_contact_person DROP COLUMN role;

ALTER TABLE tbl_invoice_item
    DROP COLUMN IF EXISTS is_final;

DROP INDEX IF EXISTS idx_tpl_setting_global_default;
DROP INDEX IF EXISTS idx_tpl_setting_invoice_unique;

DROP TABLE IF EXISTS tbl_template_setting;
DROP TABLE IF EXISTS tbl_template;

ALTER TABLE tbl_map_invoice_section
    ADD COLUMN IF NOT EXISTS template_id UUID;

-- +goose StatementEnd