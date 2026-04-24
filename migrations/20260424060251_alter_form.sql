-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form 
ALTER COLUMN clinic_id DROP NOT NULL;

ALTER TYPE calculation_method ADD VALUE IF NOT EXISTS 'EXPENSE_ENTRY';

ALTER TABLE tbl_form_field  
ADD COLUMN business_use DOUBLE PRECISION NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_field 
DROP COLUMN IF EXISTS business_use;

DROP TYPE calculation_method ADD VALUE IF NOT EXISTS 'EXPENSE_ENTRY';


ALTER TABLE tbl_form 
ALTER COLUMN clinic_id SET NOT NULL;

-- +goose StatementEnd
