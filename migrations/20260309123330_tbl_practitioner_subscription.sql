-- +goose Up
-- +goose StatementBegin
CREATE TYPE practitioner_subscription_status AS ENUM ('ACTIVE', 'PAST_DUE', 'CANCELLED', 'PAUSED', 'EXPIRED');

CREATE TABLE IF NOT EXISTS tbl_practitioner_subscription (
    id SERIAL PRIMARY KEY,
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner (id),
    subscription_id INTEGER NOT NULL,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    status practitioner_subscription_status NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_practitioner_subscription;

DROP TYPE IF EXISTS practitioner_subscription_status;
-- +goose StatementEnd