-- +goose Up
-- +goose StatementBegin

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
    mapping_id UUID NULL, 
    primary_color VARCHAR(20) NOT NULL DEFAULT '#1f4e5f',
    accent_color VARCHAR(20) NOT NULL DEFAULT '#1f4e5f',
    body_font_family VARCHAR(100) NOT NULL DEFAULT 'Arial',
    header_font_family VARCHAR(100) NOT NULL DEFAULT 'Arial',
    is_logo BOOLEAN NOT NULL DEFAULT FALSE,
    logo_id UUID NULL REFERENCES tbl_document(id),
    letterhead_id UUID NULL REFERENCES tbl_document(id),
    footer_id UUID NULL REFERENCES tbl_document(id),
    terms_text TEXT NULL,
    is_watermark BOOLEAN NOT NULL DEFAULT FALSE,
    watermark_text VARCHAR(100) NULL,
    is_tax BOOLEAN NOT NULL DEFAULT TRUE,
    table_style VARCHAR(100) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS tbl_invoice_template_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NULL,
    invoice_id UUID NULL,
    template_id UUID NOT NULL REFERENCES tbl_template(id),
    setting_id UUID NOT NULL REFERENCES tbl_template_setting(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_inv_tpl_map_lookup 
    ON tbl_invoice_template_mapping(clinic_id, invoice_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tpl_setting_mapping 
    ON tbl_template_setting(mapping_id)
    WHERE deleted_at IS NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tpl_setting_mapping;
DROP INDEX IF EXISTS idx_inv_tpl_map_lookup;

DROP TABLE IF EXISTS tbl_invoice_template_mapping;
DROP TABLE IF EXISTS tbl_template_setting;
DROP TABLE IF EXISTS tbl_template;

-- +goose StatementEnd