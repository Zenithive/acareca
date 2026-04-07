-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_subscription
    ADD COLUMN stripe_product_id TEXT,
    ADD COLUMN stripe_price_id   TEXT;

ALTER TABLE tbl_practitioner
    ADD COLUMN stripe_customer_id TEXT;

ALTER TABLE tbl_practitioner_subscription
    ADD COLUMN stripe_subscription_id TEXT,
    ADD COLUMN stripe_invoice_id       TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_subscription
    DROP COLUMN IF EXISTS stripe_product_id,
    DROP COLUMN IF EXISTS stripe_price_id;

ALTER TABLE tbl_practitioner
    DROP COLUMN IF EXISTS stripe_customer_id;

ALTER TABLE tbl_practitioner_subscription
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS stripe_invoice_id;
-- +goose StatementEnd
