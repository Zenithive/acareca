-- +goose Up
-- +goose StatementBegin

ALTER TABLE tbl_invoice
    ADD COLUMN IF NOT EXISTS contact_id UUID;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_invoice
    DROP COLUMN IF EXISTS contact_id;

-- +goose StatementEnd
