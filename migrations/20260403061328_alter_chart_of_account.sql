-- +goose Up
-- +goose StatementBegin

-- management_fee_gross_up , account_tax_id = 2
UPDATE tbl_chart_of_accounts SET account_tax_id = 2 WHERE key = 'management_fee_gross_up';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE tbl_chart_of_accounts SET account_tax_id = NULL WHERE key = 'management_fee_gross_up';
-- +goose StatementEnd
