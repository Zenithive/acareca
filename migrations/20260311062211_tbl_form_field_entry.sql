-- +goose Up
-- +goose StatementBegin
CREATE TYPE section_type AS ENUM('COLLECTION', 'COST', 'OTHER_COST');

CREATE TYPE payment_responsibility AS ENUM('OWNER', 'CLINIC');

CREATE TYPE tax_type AS ENUM('INCLUSIVE', 'EXCLUSIVE', 'MANUAL');

CREATE TYPE entry_status AS ENUM ('DRAFT', 'SUBMITTED');

CREATE TABLE IF NOT EXISTS tbl_form_field(
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_version_id UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    label VARCHAR(255) NOT NULL,
    section_type section_type NOT NULL,

    payment_responsibility payment_responsibility NOT NULL,
    tax_type tax_type NOT NULL,

    coa_id UUID NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_form_entry(
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    form_version_id UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    clinic_id UUID NOT NULL,
    submitted_by UUID NULL,
    submitted_at TIMESTAMPTZ NULL,
    status entry_status NOT NULL DEFAULT 'DRAFT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_form_entry_value(
    id UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    entry_id UUID NOT NULL REFERENCES tbl_form_entry(id) ON DELETE CASCADE,
    form_field_id UUID NOT NULL REFERENCES tbl_form_field(id) ON DELETE CASCADE,
    net_amount DECIMAL(10, 2) NULL,
    gst_amount DECIMAL(10, 2) NULL,
    gross_amount DECIMAL(10, 2) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(entry_id, form_field_id)
);

CREATE INDEX IF NOT EXISTS idx_tbl_form_field_form_version_id ON tbl_form_field(form_version_id);

CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_form_version_id ON tbl_form_entry(form_version_id);
CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_clinic_id ON tbl_form_entry(clinic_id);
CREATE INDEX IF NOT EXISTS idx_tbl_form_entry_value_entry_id ON tbl_form_entry_value(entry_id);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_form_field;
DROP TYPE IF EXISTS section_type;
DROP TYPE IF EXISTS payment_responsibility;
DROP TYPE IF EXISTS tax_type;
DROP TABLE IF EXISTS tbl_form_entry_value;
DROP TABLE IF EXISTS tbl_form_entry;
DROP TYPE IF EXISTS entry_status;
-- +goose StatementEnd
