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

CREATE TABLE IF NOT EXISTS tbl_feature_category (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS tbl_feature (
    id              SERIAL PRIMARY KEY,
    category_id     INT NOT NULL REFERENCES tbl_feature_category(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_plan_permission (
    id              SERIAL PRIMARY KEY,
    feature_id      INT NOT NULL REFERENCES tbl_feature(id) ON DELETE CASCADE,
    key             VARCHAR(100) NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_subscription_permission (
    id              SERIAL PRIMARY KEY,
    subscription_id INT NOT NULL REFERENCES tbl_subscription(id) ON DELETE CASCADE,
    permission_id   INT NOT NULL REFERENCES tbl_plan_permission(id) ON DELETE CASCADE,
    is_enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    usage_limit     INT DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT uq_sub_permission_mapping UNIQUE (subscription_id, permission_id)
);

INSERT INTO tbl_subscription (name, description, price, duration_days, is_active) VALUES 
    ('Trial', 'Trial subscription', 0, 30, TRUE),
    ('Starter', 'Starter subscription', 100, 30, TRUE),
    ('Pro', 'Pro subscription', 200, 30, TRUE),
    ('Enterprise', 'Enterprise subscription', 300, 30, TRUE);

INSERT INTO tbl_feature_category (name) VALUES ('resource_limits')
ON CONFLICT (name) DO NOTHING;

INSERT INTO tbl_feature (category_id, name)
SELECT id, 'Clinics' FROM tbl_feature_category WHERE name = 'resource_limits' UNION ALL
SELECT id, 'Forms' FROM tbl_feature_category WHERE name = 'resource_limits' UNION ALL
SELECT id, 'Transactions' FROM tbl_feature_category WHERE name = 'resource_limits' UNION ALL
SELECT id, 'Users' FROM tbl_feature_category WHERE name = 'resource_limits'
ON CONFLICT DO NOTHING;

INSERT INTO tbl_plan_permission (feature_id, key)
SELECT id, 'clinic.create' FROM tbl_feature WHERE name = 'Clinics' UNION ALL
SELECT id, 'form.create' FROM tbl_feature WHERE name = 'Forms' UNION ALL
SELECT id, 'transaction.create' FROM tbl_feature WHERE name = 'Transactions' UNION ALL
SELECT id, 'user.invite' FROM tbl_feature WHERE name = 'Users'
ON CONFLICT (key) DO NOTHING;

INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Trial'), id, TRUE,
    CASE key
        WHEN 'clinic.create'      THEN 1
        WHEN 'form.create'        THEN 1
        WHEN 'transaction.create' THEN 4
        WHEN 'user.invite'        THEN 0
    END
FROM tbl_plan_permission WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Starter'), id, TRUE,
    CASE key
        WHEN 'clinic.create'      THEN 3
        WHEN 'form.create'        THEN 5
        WHEN 'transaction.create' THEN 50
        WHEN 'user.invite'        THEN 1
    END
FROM tbl_plan_permission WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT (SELECT id FROM tbl_subscription WHERE name = 'Pro'), id, TRUE, -1
FROM tbl_plan_permission WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT (SELECT id FROM tbl_subscription WHERE name = 'Enterprise'), id, TRUE, -1
FROM tbl_plan_permission WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

ALTER TABLE tbl_practitioner_subscription
    ADD CONSTRAINT uq_practitioner_subscription_stripe_sub_id UNIQUE (stripe_subscription_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_practitioner_subscription
    DROP CONSTRAINT IF EXISTS uq_practitioner_subscription_stripe_sub_id;

DROP TABLE IF EXISTS tbl_subscription_permission;
DROP TABLE IF EXISTS tbl_plan_permission;
DROP TABLE IF EXISTS tbl_feature;
DROP TABLE IF EXISTS tbl_feature_category;
DROP TABLE IF EXISTS tbl_subscription;
-- +goose StatementEnd