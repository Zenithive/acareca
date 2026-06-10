-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_subscription (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    price         DECIMAL(12, 2) NOT NULL DEFAULT 0,
    duration_days INTEGER NOT NULL DEFAULT 30,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

INSERT INTO tbl_subscription (name, description, price, duration_days, is_active) VALUES ('Trial', 'Trial subscription', 0, 30, TRUE);
INSERT INTO tbl_subscription (name, description, price, duration_days, is_active) VALUES ('Starter', 'Starter subscription', 100, 30, TRUE);
INSERT INTO tbl_subscription (name, description, price, duration_days, is_active) VALUES ('Pro', 'Pro subscription', 200, 30, TRUE);
INSERT INTO tbl_subscription (name, description, price, duration_days, is_active) VALUES ('Enterprise', 'Enterprise subscription', 300, 30, TRUE);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_subscription;
-- +goose StatementEnd
