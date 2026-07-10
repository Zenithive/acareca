-- +goose Up
-- +goose StatementBegin

ALTER TABLE tbl_map_invoice_section 
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS account_name,
    DROP COLUMN IF EXISTS bsb_number,
    DROP COLUMN IF EXISTS account_number;

ALTER TABLE tbl_clinic_contact_person 
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS account_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS bsb_number VARCHAR(20),
    ADD COLUMN IF NOT EXISTS account_number VARCHAR(50);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_map_invoice_section 
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(100),
    ADD COLUMN IF NOT EXISTS account_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS bsb_number VARCHAR(20),
    ADD COLUMN IF NOT EXISTS account_number VARCHAR(50);

ALTER TABLE tbl_clinic_contact_person 
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS account_name,
    DROP COLUMN IF EXISTS bsb_number,
    DROP COLUMN IF EXISTS account_number;

-- +goose StatementEnd