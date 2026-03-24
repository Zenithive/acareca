-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form
    ADD COLUMN IF NOT EXISTS super_component DECIMAL(5, 2) NULL
        CONSTRAINT chk_super_component_range CHECK (super_component IS NULL OR (super_component >= 0 AND super_component <= 100));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_form DROP COLUMN IF EXISTS super_component;
-- +goose StatementEnd
