-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'enum_payment_status'
    ) THEN
        CREATE TYPE enum_payment_status AS ENUM (
            'PENDING',
            'UNPAID',
            'ACTIVE',
            'EXPIRED',
            'CANCELLED'
        );
    END IF;
END $$;

ALTER TABLE tbl_practitioner_subscription
ADD COLUMN IF NOT EXISTS payment_status enum_payment_status DEFAULT 'PENDING' NOT NULL;

-- Update existing records to have PENDING status
UPDATE tbl_practitioner_subscription 
SET payment_status = 'PENDING' 
WHERE payment_status IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_practitioner_subscription 
DROP COLUMN payment_status;
-- +goose StatementEnd
