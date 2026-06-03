-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_template (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NULL,
    html        BYTEA        NOT NULL,
    css         BYTEA        NOT NULL,
    is_default  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,

    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NULL,
    deleted_at  TIMESTAMPTZ  NULL
);

CREATE TABLE IF NOT EXISTS tbl_template_setting (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES tbl_template(id),
    primary_color VARCHAR(20) NOT NULL DEFAULT '#4247e7',
    accent_color VARCHAR(20) NOT NULL DEFAULT '#000000',
    body_font_family VARCHAR(100) NOT NULL DEFAULT 'Inter',
    header_font_family VARCHAR(100) NOT NULL DEFAULT 'Inter',
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
ALTER TABLE tbl_template_setting
ADD CONSTRAINT uq_template_setting_template_id UNIQUE (template_id);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_template_setting;
DROP TABLE IF EXISTS tbl_template;
ALTER TABLE tbl_template_setting DROP CONSTRAINT IF EXISTS uq_template_setting_template_id;
-- +goose StatementEnd
