
-- CREATE TABLE IF NOT EXISTS tbl_feature_category (
--     id          SERIAL PRIMARY KEY,
--     name        VARCHAR(100) NOT NULL UNIQUE
-- );
-- CREATE TABLE IF NOT EXISTS tbl_feature (
--     id              SERIAL PRIMARY KEY,
--     category_id     INT NOT NULL REFERENCES tbl_feature_category(id),
--     name            VARCHAR(255) NOT NULL,
--     created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     deleted_at       TIMESTAMPTZ

-- );
-- CREATE TABLE IF NOT EXISTS tbl_plan_permission (
--     id              SERIAL PRIMARY KEY,
--     feature_id      INT NOT NULL REFERENCES tbl_feature(id),
--     key             VARCHAR(100) NOT NULL UNIQUE,
--     created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     deleted_at       TIMESTAMPTZ
-- );
-- CREATE TABLE IF NOT EXISTS tbl_subscription_permission (
--     id              SERIAL PRIMARY KEY,
--     subscription_id INT NOT NULL REFERENCES tbl_subscription(id),
--     -- user_id         UUID NOT NULL,
--     permission_id   INT NOT NULL REFERENCES tbl_plan_permission(id),
--     is_enabled      BOOLEAN NOT NULL DEFAULT TRUE,
--     limit           INT DEFAULT 1,
--     created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
--     deleted_at       TIMESTAMPTZ,
--     UNIQUE (subscription_id, permission_id)
-- );

-- DROP TABLE IF EXISTS tbl_feature_category;
-- DROP TABLE IF EXISTS tbl_feature;
-- DROP TABLE IF EXISTS tbl_plan_permission;
-- DROP TABLE IF EXISTS tbl_subscription_permission;


-- +goose Up
CREATE TABLE IF NOT EXISTS tbl_feature_category (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS tbl_feature (
    id              SERIAL PRIMARY KEY,
    category_id     INT NOT NULL REFERENCES tbl_feature_category(id),
    name            VARCHAR(255) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_plan_permission (
    id              SERIAL PRIMARY KEY,
    feature_id      INT NOT NULL REFERENCES tbl_feature(id),
    key             VARCHAR(100) NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tbl_subscription_permission (
    id              SERIAL PRIMARY KEY,
    subscription_id INT NOT NULL REFERENCES tbl_subscription(id),
    permission_id   INT NOT NULL REFERENCES tbl_plan_permission(id),
    is_enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    usage_limit     INT DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (subscription_id, permission_id)
);

-- +goose Down
DROP TABLE IF EXISTS tbl_subscription_permission;
DROP TABLE IF EXISTS tbl_plan_permission;
DROP TABLE IF EXISTS tbl_feature;
DROP TABLE IF EXISTS tbl_feature_category;