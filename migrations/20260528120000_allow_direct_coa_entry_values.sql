-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS chk_form_entry_value_field_or_coa;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT chk_form_entry_value_field_or_coa
CHECK (
    form_field_id IS NOT NULL
    OR coa_id IS NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form_entry_value
DROP CONSTRAINT IF EXISTS chk_form_entry_value_field_or_coa;

ALTER TABLE tbl_form_entry_value
ADD CONSTRAINT chk_form_entry_value_field_or_coa
CHECK (
    (form_field_id IS NOT NULL AND coa_id IS NULL)
    OR (form_field_id IS NULL AND coa_id IS NOT NULL)
);
-- +goose StatementEnd
