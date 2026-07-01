-- +goose Up
-- +goose StatementBegin

-- Add unique constraint on stripe_subscription_id for upsert logic
ALTER TABLE tbl_practitioner_subscription 
ADD CONSTRAINT uq_stripe_subscription_id UNIQUE (stripe_subscription_id);

-- Add index to improve performance for expiry queries
CREATE INDEX idx_subscription_expiry 
ON tbl_practitioner_subscription (status, end_date) 
WHERE deleted_at IS NULL;

-- Add index to improve performance for active subscription queries
CREATE INDEX idx_subscription_active 
ON tbl_practitioner_subscription (practitioner_id, status, start_date, end_date) 
WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_subscription_active;
DROP INDEX IF EXISTS idx_subscription_expiry;
ALTER TABLE tbl_practitioner_subscription DROP CONSTRAINT IF EXISTS uq_stripe_subscription_id;

-- +goose StatementEnd
