-- +goose Up
-- +goose StatementBegin

CREATE TYPE practitioner_subscription_status AS ENUM ('PENDING', 'COMPLETE');

ALTER TABLE tbl_practitioner
    ADD COLUMN IF NOT EXISTS subscription_status practitioner_subscription_status NOT NULL DEFAULT 'PENDING';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tbl_practitioner DROP COLUMN IF EXISTS subscription_status;

DROP TYPE IF EXISTS practitioner_subscription_status;

-- +goose StatementEnd
