-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_practitioner_subscription
    ADD CONSTRAINT uq_practitioner_subscription_stripe_sub_id
    UNIQUE (stripe_subscription_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_practitioner_subscription
    DROP CONSTRAINT IF EXISTS uq_practitioner_subscription_stripe_sub_id;
-- +goose StatementEnd
