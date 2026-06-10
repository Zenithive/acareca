-- +goose Up
-- +goose StatementBegin

-- Feature category
INSERT INTO tbl_feature_category (name) VALUES ('resource_limits')
ON CONFLICT (name) DO NOTHING;

-- Features
INSERT INTO tbl_feature (category_id, name)
SELECT id, 'Clinics' FROM tbl_feature_category WHERE name = 'resource_limits'
ON CONFLICT DO NOTHING;

INSERT INTO tbl_feature (category_id, name)
SELECT id, 'Forms' FROM tbl_feature_category WHERE name = 'resource_limits'
ON CONFLICT DO NOTHING;

INSERT INTO tbl_feature (category_id, name)
SELECT id, 'Transactions' FROM tbl_feature_category WHERE name = 'resource_limits'
ON CONFLICT DO NOTHING;

INSERT INTO tbl_feature (category_id, name)
SELECT id, 'Users' FROM tbl_feature_category WHERE name = 'resource_limits'
ON CONFLICT DO NOTHING;

-- Permission keys
INSERT INTO tbl_plan_permission (feature_id, key)
SELECT id, 'clinic.create' FROM tbl_feature WHERE name = 'Clinics'
ON CONFLICT (key) DO NOTHING;

INSERT INTO tbl_plan_permission (feature_id, key)
SELECT id, 'form.create' FROM tbl_feature WHERE name = 'Forms'
ON CONFLICT (key) DO NOTHING;

INSERT INTO tbl_plan_permission (feature_id, key)
SELECT id, 'transaction.create' FROM tbl_feature WHERE name = 'Transactions'
ON CONFLICT (key) DO NOTHING;

INSERT INTO tbl_plan_permission (feature_id, key)
SELECT id, 'user.invite' FROM tbl_feature WHERE name = 'Users'
ON CONFLICT (key) DO NOTHING;

-- Trial limits: clinic=1, form=1, transaction=4, user.invite=0 (blocked)
INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Trial'),
    id,
    TRUE,
    CASE key
        WHEN 'clinic.create'      THEN 1
        WHEN 'form.create'        THEN 1
        WHEN 'transaction.create' THEN 4
        WHEN 'user.invite'        THEN 0
    END
FROM tbl_plan_permission
WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

-- Starter limits: clinic=3, form=5, transaction=50, user.invite=1
INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Starter'),
    id,
    TRUE,
    CASE key
        WHEN 'clinic.create'      THEN 3
        WHEN 'form.create'        THEN 5
        WHEN 'transaction.create' THEN 50
        WHEN 'user.invite'        THEN 1
    END
FROM tbl_plan_permission
WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

-- Pro limits: -1 = unlimited
INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Pro'),
    id,
    TRUE,
    -1
FROM tbl_plan_permission
WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

-- Enterprise limits: -1 = unlimited
INSERT INTO tbl_subscription_permission (subscription_id, permission_id, is_enabled, usage_limit)
SELECT
    (SELECT id FROM tbl_subscription WHERE name = 'Enterprise'),
    id,
    TRUE,
    -1
FROM tbl_plan_permission
WHERE key IN ('clinic.create', 'form.create', 'transaction.create', 'user.invite')
ON CONFLICT (subscription_id, permission_id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM tbl_subscription_permission
WHERE permission_id IN (SELECT id FROM tbl_plan_permission WHERE key IN ('clinic.create','form.create','transaction.create','user.invite'));

DELETE FROM tbl_plan_permission WHERE key IN ('clinic.create','form.create','transaction.create','user.invite');

DELETE FROM tbl_feature WHERE name IN ('Clinics','Forms','Transactions','Users');

DELETE FROM tbl_feature_category WHERE name = 'resource_limits';
-- +goose StatementEnd
