-- +goose Up
-- +goose StatementBegin

ALTER TABLE tbl_invoice
ADD CONSTRAINT fk_invoice_template
FOREIGN KEY (template_id)
REFERENCES tbl_invoice_clinic(id);

CREATE TABLE IF NOT EXISTS tbl_template (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NULL,
    html        TEXT         NOT NULL,
    css         TEXT         NOT NULL,
    is_default  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,

    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NULL,
    deleted_at  TIMESTAMPTZ  NULL
);


CREATE TABLE IF NOT EXISTS tbl_template_setting (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID        NOT NULL REFERENCES tbl_template(id),

    primary_color       VARCHAR(20)  NOT NULL DEFAULT '#4247e7',
    accent_color        VARCHAR(20)  NOT NULL DEFAULT '#000000',
    body_font_family    VARCHAR(100) NOT NULL DEFAULT 'Inter',
    header_font_family  VARCHAR(100) NOT NULL DEFAULT 'Inter',
    is_logo           BOOLEAN      NOT NULL DEFAULT FALSE,
    logo_id            UUID         NULL REFERENCES tbl_document(id),
    letterhead_id     UUID         NULL REFERENCES tbl_document(id),
    footer_id         UUID         NULL REFERENCES tbl_document(id),
    terms_text          TEXT         NULL,
    is_watermark   BOOLEAN      NOT NULL DEFAULT FALSE,
    watermark_text      VARCHAR(100) NULL,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NULL,
    deleted_at  TIMESTAMPTZ NULL,
);



-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_template;
DROP TABLE IF EXISTS tbl_template_setting;
-- +goose StatementEnd
