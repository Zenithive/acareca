-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form 
ALTER COLUMN clinic_id DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'calculation_method') THEN
        CREATE TYPE calculation_method AS ENUM('EXPENSE_ENTRY');
    END IF;
END$$;

ALTER TABLE tbl_form_field  
ADD COLUMN bussiness_use DOUBLE PRECISION NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field 
DROP COLUMN IF EXISTS bussiness_use;

-- Drop enum safely
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'calculation_method') THEN
        DROP TYPE calculation_method;
    END IF;
END$$;


ALTER TABLE tbl_form 
ALTER COLUMN clinic_id SET NOT NULL;

-- +goose StatementEnd
