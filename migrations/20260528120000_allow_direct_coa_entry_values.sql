-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value
ALTER COLUMN form_field_id DROP NOT NULL;

ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS tbl_form_entry_value_form_field_id_fkey;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT tbl_form_entry_value_form_field_id_fkey
FOREIGN KEY (form_field_id) REFERENCES tbl_form_field(id);

ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS chk_form_entry_value_field_or_coa;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT chk_form_entry_value_field_or_coa
CHECK (
    (form_field_id IS NOT NULL) OR (coa_id IS NOT NULL)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS chk_form_entry_value_field_or_coa;

ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS tbl_form_entry_value_form_field_id_fkey;

ALTER TABLE tbl_form_entry_value
ALTER COLUMN form_field_id SET NOT NULL;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT tbl_form_entry_value_form_field_id_fkey
FOREIGN KEY (form_field_id) REFERENCES tbl_form_field(id) ON DELETE CASCADE;

-- +goose StatementEnd
